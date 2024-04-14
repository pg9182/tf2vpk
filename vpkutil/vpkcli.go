package vpkutil

import (
	"fmt"

	"github.com/pg9182/tf2vpk"
	"github.com/pg9182/tf2vpk/internal"
	"github.com/spf13/pflag"
)

// CLIIncludeExclude filters VPK files using the provided globs.
type CLIIncludeExclude struct {
	Exclude        *[]string
	ExcludeBSPLump *[]int
	Include        *[]string
}

// NewCLIIncludeExclude creates a new CLIIncludeExclude and registers it with
// the provided [pflag.FlagSet].
func NewCLIIncludeExclude(set *pflag.FlagSet) CLIIncludeExclude {
	return CLIIncludeExclude{
		Exclude:        pflag.StringSlice("exclude", nil, "Excludes files or directories matching the provided glob (anchor to the start with /)"),
		ExcludeBSPLump: pflag.IntSlice("exclude-bsp-lump", nil, "Shortcut for --exclude to remove %04x.bsp_lump"),
		Include:        pflag.StringSlice("include", nil, "Negates --exclude for files or directories matching the provided glob"),
	}
}

// Skip determines whether to skip the specified file.
func (ie CLIIncludeExclude) Skip(f tf2vpk.ValvePakFile) (bool, error) {
	var excluded bool
	for _, x := range *ie.Exclude {
		if m, err := internal.MatchGlobParents(x, f.Path); err != nil {
			return false, fmt.Errorf("process excludes: match %q against glob %q: %w", f.Path, x, err)
		} else if m {
			excluded = true
			break
		}
	}
	if !excluded {
		for _, n := range *ie.ExcludeBSPLump {
			x := fmt.Sprintf("%04x.bsp_lump", n)
			if m, err := internal.MatchGlobParents(x, f.Path); err != nil {
				return false, fmt.Errorf("process bsp lump excludes: match %q against glob %q: %w", f.Path, x, err)
			} else if m {
				excluded = true
				break
			}
		}
	}
	for _, x := range *ie.Include {
		if m, err := internal.MatchGlobParents(x, f.Path); err != nil {
			return false, fmt.Errorf("process includes: match %q against glob %q: %w", f.Path, x, err)
		} else if m {
			excluded = false
			break
		}
	}
	return excluded, nil
}

// CLIResolveOpen takes over the last 1 or 2 arguments, using them to resolve
// and open a VPK.
type CLIResolveOpen struct {
	Arg       int
	Optional  bool
	VPKPrefix *string
	set       *pflag.FlagSet
}

// NewCLIResolveOpen creates a new CLIResolveOpen and registers it with the
// provided [pflag.FlagSet]. It starts processing arguments at the provided
// index (where 0 is the first argument after flags have been parsed).
func NewCLIResolveOpen(set *pflag.FlagSet, arg int, optional bool) CLIResolveOpen {
	return CLIResolveOpen{
		Arg:       arg,
		Optional:  optional,
		VPKPrefix: pflag.StringP("vpk-prefix", "p", "english", "VPK prefix"),
		set:       set,
	}
}

// ArgHelp returns help text to add to the arguments usage.
func (ro CLIResolveOpen) ArgHelp() string {
	h := "(vpk_dir vpk_name)|vpk_path"
	if ro.Optional {
		h = "[" + h + "]"
	}
	return h
}

// ArgCheck ensures at 1-2 (or 0-2 if optional) arguments are provided at the
// specified argument index.
func (ro CLIResolveOpen) ArgCheck() bool {
	n := ro.set.NArg()
	if ro.Optional {
		return ro.Arg <= n && n <= ro.Arg+2
	}
	return ro.Arg < n && n <= ro.Arg+2
}

// Resolve resolves the VPK path.
func (ro CLIResolveOpen) Resolve() (vpk tf2vpk.ValvePakRef, err error) {
	args := ro.set.Args()
	args = args[min(len(args), ro.Arg):]
	switch len(args) {
	case 2:
		vpk, err = tf2vpk.ValvePakRef{Path: args[0], Prefix: *ro.VPKPrefix, Name: args[1]}, nil
	case 1:
		vpk, err = tf2vpk.PathToValvePakRef(args[0], *ro.VPKPrefix)
	default:
		if len(args) != 0 || !ro.Optional {
			panic("invalid argument count, expected last arguments to be (vpk_dir vpk_name)|vpk_path")
		}
	}
	if err != nil {
		err = fmt.Errorf("resolve vpk: %w", err)
		return
	}
	return
}

// ResolveOpen resolves the VPK path and opens a reader.
func (ro CLIResolveOpen) ResolveOpen() (vpk tf2vpk.ValvePakRef, r *tf2vpk.Reader, err error) {
	vpk, err = ro.Resolve()
	if err == nil && vpk != (tf2vpk.ValvePakRef{}) {
		if r, err = tf2vpk.NewReader(vpk); err != nil {
			err = fmt.Errorf("open vpk: %w", err)
		}
	}
	return
}
