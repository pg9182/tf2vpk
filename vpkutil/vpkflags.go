package vpkutil

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/pg9182/tf2vpk"
	"github.com/pg9182/tf2vpk/internal"
)

// VPKFlagsFilename is the name of the vpkflags file. It should be at the root
// of the folder to be packed.
const VPKFlagsFilename = ".vpkflags"

// VPKFlags is a list of rules for adding flags to files in a VPK. The rules are
// matched in reverse order, i.e., the last one takes effect.
type VPKFlags struct {
	rules []vpkFlagsRule
}

// vpkFlagsRule is a rule for VPKFlags.
type vpkFlagsRule struct {
	// Glob similar to the syntax used by [path.Match], but matches starting at
	// any path component unless anchored by prefixing the pattern with a "/".
	// The special glob "/" matches everything.
	//
	// Must not contain whitespace or newlines, otherwise behavior is undefined.
	//
	// See internal/util_test.go for more examples.
	Glob string

	// Load flags are the load flags to use for matching VPK entries.
	LoadFlags uint32

	// Load flags are the load flags to use for matching VPK entries.
	TextureFlags uint16
}

func isLiteralPathValidForGlobRuleGlob(path string) error {
	if strings.ContainsFunc(path, unicode.IsSpace) {
		return fmt.Errorf("path contains whitespace")
	}
	if strings.ContainsAny(path, "#") {
		return fmt.Errorf("path contains comment character")
	}
	if strings.ContainsAny(path, "?*\\[") {
		return fmt.Errorf("path contains special glob character")
	}
	return nil
}

// Add appends a new rule.
func (v *VPKFlags) Add(glob string, loadFlags uint32, textureFlags uint16) error {
	if strings.ContainsFunc(glob, unicode.IsSpace) {
		return fmt.Errorf("glob contains whitespace")
	}
	v.rules = append(v.rules, vpkFlagsRule{
		Glob:         glob,
		LoadFlags:    loadFlags,
		TextureFlags: textureFlags,
	})
	return nil
}

// GenerateExplicit generates a new VPKFlags based on the provided VPK with all
// files explicitly set, replacing any existing rules.
func (v *VPKFlags) GenerateExplicit(root tf2vpk.ValvePakDir) error {
	var rules []vpkFlagsRule
	for _, file := range root.File {
		load, err := file.LoadFlags()
		if err != nil {
			return fmt.Errorf("entry %q: compute load flags: %w", file.Path, err)
		}
		texture, err := file.TextureFlags()
		if err != nil {
			return fmt.Errorf("entry %q: compute texture flags: %w", file.Path, err)
		}
		if err := isLiteralPathValidForGlobRuleGlob(file.Path); err != nil {
			return fmt.Errorf("entry %q: cannot add to vpkflags: %w", file.Path, err)
		}
		rules = append(rules, vpkFlagsRule{
			Glob:         file.Path,
			LoadFlags:    load,
			TextureFlags: texture,
		})
	}
	v.rules = rules
	return nil
}

// Generate generates a new minimal VPKFlags (through inheriting from parent
// dirs) based on the provided VPK, replacing any existing rules.
func (v *VPKFlags) Generate(root tf2vpk.ValvePakDir) error {
	const (
		debugAddRedundantRules = false
	)
	var rules []vpkFlagsRule

	type Flags struct {
		Load    uint32
		Texture uint16
	}
	type PathFlags struct {
		Flags    Flags
		Children map[string]*PathFlags
		Freq     map[Flags]int
	}
	rootFlags := &PathFlags{
		Children: map[string]*PathFlags{},
		Freq:     map[Flags]int{},
	}

	for _, file := range root.File {
		var flags Flags
		if v, err := file.LoadFlags(); err != nil {
			return fmt.Errorf("entry %q: compute load flags: %w", file.Path, err)
		} else {
			flags.Load = v
		}
		if v, err := file.TextureFlags(); err != nil {
			return fmt.Errorf("entry %q: compute texture flags: %w", file.Path, err)
		} else {
			flags.Texture = v
		}

		segs := strings.Split(file.Path, "/")
		segFlags := rootFlags
		for i, curSeg := range segs {
			segFlags.Freq[flags]++

			curSegFlags, ok := segFlags.Children[curSeg]
			if !ok {
				curSegFlags = &PathFlags{
					Flags: flags,
				}
				if leaf := i == len(segs)-1; !leaf {
					curSegFlags.Freq = map[Flags]int{}
					curSegFlags.Children = map[string]*PathFlags{}
				}
				segFlags.Children[curSeg] = curSegFlags
			}
			segFlags = curSegFlags
		}
	}

	// consolidate flags breadth-first
	{
		queue := []*PathFlags{rootFlags}
		for len(queue) != 0 {
			var cur *PathFlags
			cur, queue = queue[0], queue[1:]

			// skip leaf nodes
			if len(cur.Children) == 0 {
				continue
			}

			// get most common leaf flag
			var (
				maxCount int
				minFlag  Flags
			)
			for fileFlags, count := range cur.Freq {
				switch {
				case count > maxCount: // take the flag with the max count amongst all children
					maxCount = count
					minFlag = fileFlags
				case count == maxCount: // if multiple with the same count, take the lowest
					if minFlag != fileFlags {
						if minFlag.Load > fileFlags.Load || (minFlag.Load == fileFlags.Load && minFlag.Texture > fileFlags.Texture) {
							minFlag = fileFlags
						}
					}
				}
			}
			cur.Flags = minFlag

			// add children to queue
			for _, child := range cur.Children {
				queue = append(queue, child)
			}
		}
	}

	// walk depth first in name order, outputting top-level flags, then exceptions for children
	var outputWalkDfs func(path string, cur, parent *PathFlags) error
	outputWalkDfs = func(path string, cur, parent *PathFlags) error {
		if debugAddRedundantRules || parent == nil || parent.Flags != cur.Flags {
			if err := isLiteralPathValidForGlobRuleGlob(path); err != nil {
				return fmt.Errorf("path %q: cannot add to vpkflags: %w", path, err)
			}
			rules = append(rules, vpkFlagsRule{
				Glob:         path,
				LoadFlags:    cur.Flags.Load,
				TextureFlags: cur.Flags.Texture,
			})
		}

		segs := make([]string, 0, len(cur.Children))
		for seg := range cur.Children {
			segs = append(segs, seg)
		}
		sort.Strings(segs)

		if !strings.HasSuffix(path, "/") {
			path += "/"
		}
		for _, seg := range segs {
			if err := outputWalkDfs(path+seg, cur.Children[seg], cur); err != nil {
				return err
			}
		}
		return nil
	}
	if err := outputWalkDfs("/", rootFlags, nil); err != nil {
		return err
	}

	v.rules = rules
	return nil
}

// Test ensures the rules match the existing flags in the provided VPK.
func (v VPKFlags) Test(root tf2vpk.ValvePakDir) error {
	for _, file := range root.File {
		load, err := file.LoadFlags()
		if err != nil {
			return fmt.Errorf("entry %q: compute load flags: %w", file.Path, err)
		}
		texture, err := file.TextureFlags()
		if err != nil {
			return fmt.Errorf("entry %q: compute texture flags: %w", file.Path, err)
		}
		loadAct, textureAct, rule := v.match(file.Path)
		if load != loadAct {
			if rule != -1 {
				return fmt.Errorf("entry %q: has load flags %032b, vpkflags has incorrect %032b (rule %d: %s)", file.Path, load, loadAct, rule, v.rules[rule])
			}
			return fmt.Errorf("entry %q: has load flags %032b, vpkflags has incorrect %032b (no rule matched)", file.Path, load, loadAct)
		}
		if texture != textureAct {
			if rule != -1 {
				return fmt.Errorf("entry %q: has texture flags %016b, vpkflags has incorrect %016b (rule %d: %s)", file.Path, texture, textureAct, rule, v.rules[rule])
			}
			return fmt.Errorf("entry %q: has texture flags %016b, vpkflags has incorrect %016b (no rule matched)", file.Path, texture, textureAct)
		}
	}
	return nil
}

// Match returns the load and texture flags for the provided path.
func (v VPKFlags) Match(path string) (loadFlags uint32, textureFlags uint16) {
	loadFlags, textureFlags, _ = v.match(path)
	return
}

func (v VPKFlags) match(path string) (loadFlags uint32, textureFlags uint16, rule int) {
	rule = -1
	for i := len(v.rules) - 1; i >= 0; i-- {
		if m, _ := internal.MatchGlobParents(v.rules[i].Glob, path); m {
			loadFlags = v.rules[i].LoadFlags
			textureFlags = v.rules[i].TextureFlags
			rule = i
			break
		}
	}
	return
}

// String returns a string which can later be parsed by Parse.
func (v VPKFlags) String() string {
	var b strings.Builder

	pathLen := 64
	for _, rule := range v.rules {
		pathLen = max(pathLen, len(rule.Glob))
	}

	fmt.Fprintf(&b, "%-32s %-16s %*s # %s\n", "# load flags", "texture flags", -pathLen, "path (last match wins, / to anchor, * supported)", "human-readable description (ignored)")
	for _, rule := range v.rules {
		fmt.Fprintf(&b, "%032b %016b %*s # load=%s texture=%s\n",
			rule.LoadFlags, rule.TextureFlags, -pathLen, rule.Glob,
			tf2vpk.DescribeLoadFlags(rule.LoadFlags),
			tf2vpk.DescribeTextureFlags(rule.TextureFlags),
		)
	}
	return b.String()
}

func (v vpkFlagsRule) String() string {
	return fmt.Sprintf("%032b %016b %s", v.LoadFlags, v.TextureFlags, v.Glob)
}

// Parse parses a vpkflags string, replacing any existing rules.
func (v *VPKFlags) Parse(s string) error {
	var rules []vpkFlagsRule
	var lineNo int

	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {

		// get the line
		line := sc.Text()
		lineNo++

		// cut the comment
		line, _, _ = strings.Cut(line, "#")

		// trim whitespace and remove multiple consecutive whitespace, split into fields
		fields := strings.Fields(line)

		// skip empty (or comment-only) lines
		if len(fields) == 0 {
			continue
		}

		// check field count
		if len(fields) < 3 {
			return fmt.Errorf("line %d: expected 3 fields (load_flags texture_flags glob), got %d", lineNo, len(fields))
		}
		if len(fields) > 3 {
			return fmt.Errorf("line %d: expected 3 fields (load_flags texture_flags glob), got %d (note that the glob must not contain whitespace)", lineNo, len(fields))
		}

		// create the rule
		rule := vpkFlagsRule{
			Glob: fields[2],
		}

		// parse load flags
		if v, err := strconv.ParseUint(fields[0], 2, 32); err != nil {
			return fmt.Errorf("line %d: parse load flags binary %q: %w", lineNo, fields[0], err)
		} else {
			rule.LoadFlags = uint32(v)
		}

		// parse texture flags
		if v, err := strconv.ParseUint(fields[1], 2, 16); err != nil {
			return fmt.Errorf("line %d: parse texture flags binary %q: %w", lineNo, fields[1], err)
		} else {
			rule.TextureFlags = uint16(v)
		}

		// add the rule
		rules = append(rules, rule)
	}
	if err := sc.Err(); err != nil {
		return err
	}

	v.rules = rules
	return nil
}

// ParseFile is like Parse, but reads from a file.
func (v *VPKFlags) ParseFile(name string) error {
	buf, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	return v.Parse(string(buf))
}
