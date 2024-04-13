package vpkutil

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/pg9182/tf2vpk"
	"github.com/pg9182/tf2vpk/internal"
)

// VPKIgnoreFilename is the name of the vpkignore file. It should be at the root
// of the folder to be packed.
const VPKIgnoreFilename = ".vpkignore"

// VPKIgnore is a list of patterns to ignore when packing a VPK file in a
// similar fashion to gitignore.
//
// Each pattern consists of a case-sensitive glob (same as with VPKFlags),
// optionally negated (denoted by a exclamation mark prefix in string form).
//
// The rules are checked from top to bottom. If a rule is matched, the file is
// tentatively excluded unless matched by a negated rule afterwards (a matched
// negated rule before a match does nothing). If matched by a negated rule, the
// file is not excluded, and no further rules for that file are processed.
type VPKIgnore struct {
	rules []vpkIgnoreRule
}

type vpkIgnoreRule struct {
	Glob   string
	Negate bool
}

// AddDefault adds some general rules.
func (v *VPKIgnore) AddDefault() {
	v.rules = append(v.rules, vpkIgnoreRule{"*.vpk", false})
	v.rules = append(v.rules, vpkIgnoreRule{"/.vpk*", false})
	v.rules = append(v.rules, vpkIgnoreRule{".nfs*", false})
	v.rules = append(v.rules, vpkIgnoreRule{".directory", false})
	v.rules = append(v.rules, vpkIgnoreRule{".Trash-*", false})
	v.rules = append(v.rules, vpkIgnoreRule{"System Volume Information", false})
	v.rules = append(v.rules, vpkIgnoreRule{"Thumbs.db", false})
	v.rules = append(v.rules, vpkIgnoreRule{"Thumbs.db:encryptable", false})
	v.rules = append(v.rules, vpkIgnoreRule{"Desktop.ini", false})
	v.rules = append(v.rules, vpkIgnoreRule{"ehthumbs.db", false})
	v.rules = append(v.rules, vpkIgnoreRule{"ehthumbs_vista.db", false})
	v.rules = append(v.rules, vpkIgnoreRule{".DS_Store", false})
	v.rules = append(v.rules, vpkIgnoreRule{".AppleDouble", false})
	v.rules = append(v.rules, vpkIgnoreRule{".LSOverride", false})
	v.rules = append(v.rules, vpkIgnoreRule{".DocumentRevisions-V100", false})
	v.rules = append(v.rules, vpkIgnoreRule{".fseventsd", false})
	v.rules = append(v.rules, vpkIgnoreRule{".Spotlight-V100", false})
	v.rules = append(v.rules, vpkIgnoreRule{".TemporaryItems", false})
	v.rules = append(v.rules, vpkIgnoreRule{".Trashes", false})
	v.rules = append(v.rules, vpkIgnoreRule{".VolumeIcon.icns", false})
	v.rules = append(v.rules, vpkIgnoreRule{".com.apple.timemachine.donotpresent", false})
	v.rules = append(v.rules, vpkIgnoreRule{".vscode", false})
	v.rules = append(v.rules, vpkIgnoreRule{".idea", false})
	v.rules = append(v.rules, vpkIgnoreRule{".git*", false})
	v.rules = append(v.rules, vpkIgnoreRule{".fr-*", false})
	v.rules = append(v.rules, vpkIgnoreRule{"[._]*.s[a-v][a-z]", false})
	v.rules = append(v.rules, vpkIgnoreRule{"*.swp", false})
	v.rules = append(v.rules, vpkIgnoreRule{"*.part", false})
	v.rules = append(v.rules, vpkIgnoreRule{"._*", false})
	v.rules = append(v.rules, vpkIgnoreRule{"~*", false})
	v.rules = append(v.rules, vpkIgnoreRule{"*~", false})
	v.rules = append(v.rules, vpkIgnoreRule{".example_for_negated_rules_*", false})
	v.rules = append(v.rules, vpkIgnoreRule{".example_for_negated_rules_include_me", true})
}

// Add adds a rule.
func (v *VPKIgnore) Add(glob string, negate bool) error {
	if strings.HasPrefix(glob, "!") {
		return fmt.Errorf("glob starts with negation character")
	}
	if strings.ContainsAny(glob, "#") {
		return fmt.Errorf("glob contains comment character")
	}
	if strings.ContainsAny(glob, "\n\r") {
		return fmt.Errorf("glob contains newlines or carriage returns")
	}
	if strings.TrimSpace(glob) != glob {
		return fmt.Errorf("glob contains leading or trailing whitespace")
	}
	v.rules = append(v.rules, vpkIgnoreRule{
		Glob:   glob,
		Negate: negate,
	})
	return nil
}

// AddAutoExclusions automatically adds negated rules for files in a VPK.
func (v *VPKIgnore) AddAutoExclusions(root tf2vpk.ValvePakDir) error {
	for _, file := range root.File {
		if v.Match(file.Path) {
			if strings.ContainsAny(file.Path, "?*\\[") {
				return fmt.Errorf("entry %q: path contains special glob character", file.Path)
			}
			if err := v.Add("/"+file.Path, true); err != nil {
				return fmt.Errorf("entry %q: %w", file.Path, err)
			}
		}
	}
	return nil
}

// Match checks whether the provided path should be ignored.
func (v VPKIgnore) Match(path string) bool {
	var excluding bool
	for _, rule := range v.rules {
		if excluding != rule.Negate {
			continue // don't process negations until we see an exclusion, then don't process anything but negations
		}
		if m, _ := internal.MatchGlobParents(rule.Glob, path); m {
			if rule.Negate {
				return false
			}
			excluding = !rule.Negate
		}
	}
	return excluding
}

// String returns a string which can later be parsed by Parse.
func (v VPKIgnore) String() string {
	var b strings.Builder
	b.WriteString("# list of glob patterns to be excluded when repacking the vpk\n")
	b.WriteString("# - use a leading slash anchor the path\n")
	b.WriteString("# - use a exclamation mark prefix to negate the pattern\n")
	b.WriteString("# - patterns are scanned from start to end\n")
	b.WriteString("# - the first matched rule sets the file as excluded unless a negated rule afterwards also matches it\n")
	b.WriteString("# - no further rules are matched after a negated exclusion\n")
	b.WriteString("\n")
	for _, rule := range v.rules {
		b.WriteString(rule.String())
		b.WriteByte('\n')
	}
	return b.String()
}

func (v vpkIgnoreRule) String() string {
	if v.Negate {
		return "!" + v.Glob
	}
	return v.Glob
}

// Parse parses a vpkignore string, replacing any existing rules.
func (v *VPKIgnore) Parse(s string) error {
	var rules []vpkIgnoreRule
	var lineNo int

	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {

		// get the line
		line := sc.Text()
		lineNo++

		// cut the comment
		line, _, _ = strings.Cut(line, "#")

		// trim whitespace
		line = strings.TrimSpace(line)

		// skip empty (or comment-only) lines
		if len(line) == 0 {
			continue
		}

		// get glob, check if negated, trim again
		glob, negate := strings.CutPrefix(line, "!")
		glob = strings.TrimSpace(glob)

		// add the rule
		rules = append(rules, vpkIgnoreRule{
			Glob:   glob,
			Negate: negate,
		})
	}
	if err := sc.Err(); err != nil {
		return err
	}

	v.rules = rules
	return nil
}

// ParseFile is like Parse, but reads from a file.
func (v *VPKIgnore) ParseFile(name string) error {
	buf, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	return v.Parse(string(buf))
}
