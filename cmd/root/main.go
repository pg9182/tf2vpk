package root

import (
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	"slices"
	"strings"

	"github.com/pg9182/tf2vpk"
	"github.com/pg9182/tf2vpk/internal"
	"github.com/spf13/cobra"
)

var Flags struct {
	VPKDir    string
	VPKPrefix string
	Threads   int
}

var Command = &cobra.Command{
	Use:   "tf2vpk",
	Short: "Manipulates Respawn VPK archives",
	PreRun: func(cmd *cobra.Command, args []string) {
		if Flags.Threads < 0 {
			Flags.Threads = 0
		}
		if Flags.Threads > runtime.NumCPU() {
			runtime.GOMAXPROCS(Flags.Threads)
		}
	},
}

var GroupVPKRead = &cobra.Group{
	ID:    "vpk-read",
	Title: "Read Commands:",
}

var GroupVPKWrite = &cobra.Group{
	ID:    "vpk-write",
	Title: "Update Commands:",
}

var GroupVPKRepack = &cobra.Group{
	ID:    "vpk-repack",
	Title: "Repack Commands:",
}

func init() {
	Command.AddGroup(GroupVPKRead, GroupVPKWrite, GroupVPKRepack)
	Command.PersistentFlags().StringVar(&Flags.VPKDir, "vpk-dir", "", "set the vpk directory, and use vpk names instead of paths")
	Command.PersistentFlags().StringVar(&Flags.VPKPrefix, "vpk-prefix", "english", "the vpk locale prefix to use")
	Command.PersistentFlags().IntVarP(&Flags.Threads, "threads", "j", runtime.NumCPU(), "number of threads to use for decompression (-1 to disable, default is cpu count)")
}

// VPK resolves the provided name to a VPK.
func VPK(name string) (tf2vpk.ValvePakRef, error) {
	if Flags.VPKDir != "" {
		if name == "" {
			return tf2vpk.ValvePakRef{}, fmt.Errorf("invalid vpk name %q", name)
		}
		return tf2vpk.ValvePakRef{
			Path:   Flags.VPKDir,
			Prefix: Flags.VPKPrefix,
			Name:   name,
		}, nil
	}
	if vpk, err := tf2vpk.PathToValvePakRef(name, Flags.VPKPrefix); err != nil {
		return tf2vpk.ValvePakRef{}, fmt.Errorf("invalid vpk path %q: %w", name, err)
	} else {
		return vpk, nil
	}
}

// ArgVPK updates cmd to use the vpk name/path as the first mandatory argument,
// validating it and registering completions.
//
// If i is positive, it completes arguments after (one or multi) it with names
// from the VPK (these are not validated).
func ArgVPK(out *tf2vpk.ValvePakRef, cmd *cobra.Command, i int, multi, dirs, files bool) {
	if i == 0 {
		panic("file arg index must not be zero")
	}

	// check the help text if it's set
	if a, b, _ := strings.Cut(cmd.Use, " "); a != "" {
		if a, _, _ := strings.Cut(b, " "); a != "vpk_path" {
			panic("second argument help must be vpk_path")
		}
	}

	// add the argument validation/parsing
	args := func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			if Flags.VPKDir != "" {
				return fmt.Errorf("vpk name is required")
			}
			return fmt.Errorf("vpk path is required")
		}
		if vpk, err := VPK(args[0]); err != nil {
			return err
		} else {
			*out = vpk
		}
		return nil
	}
	if next := cmd.Args; next != nil {
		cmd.Args = cobra.MatchAll(args, next)
	} else {
		cmd.Args = args
	}

	// add the argument completion
	if validArgsFunction, next := func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			if Flags.VPKDir == "" {
				return []string{tf2vpk.Ext}, cobra.ShellCompDirectiveFilterFileExt
			}
			ds, err := os.ReadDir(Flags.VPKDir)
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}
			var ns []string
			for _, d := range ds {
				if n, idx, err := tf2vpk.SplitName(d.Name(), Flags.VPKPrefix); err == nil && idx == tf2vpk.ValvePakIndexDir {
					ns = append(ns, n)
				}
			}
			return ns, cobra.ShellCompDirectiveNoFileComp
		}
		if i > 0 && len(args) >= i && (multi || len(args) == i) {
			return ArgVPKFileCompletions(args, toComplete, dirs, files)
		}
		return nil, cobra.ShellCompDirectiveDefault
	}, cmd.ValidArgsFunction; next != nil {
		cmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 || (i > 0 && len(args) >= i && (multi || len(args) == i)) {
				return validArgsFunction(cmd, args, toComplete)
			}
			return next(cmd, args, toComplete)
		}
	} else {
		cmd.ValidArgsFunction = validArgsFunction
	}
}

// ArgVPKFileCompletions returns file completions for a VPK.
func ArgVPKFileCompletions(args []string, toComplete string, dirs, files bool) ([]string, cobra.ShellCompDirective) {
	vpk, err := VPK(args[0])
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	r, err := os.Open(vpk.Resolve(tf2vpk.ValvePakIndexDir))
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	defer r.Close()

	var root tf2vpk.ValvePakDir
	if err := root.Deserialize(io.NewSectionReader(r, 0, 1<<63-1)); err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var (
		cs []string
		ds = map[string]struct{}{}
	)
	for _, f := range root.File {
		if files {
			if strings.HasPrefix(f.Path, toComplete) {
				cs = append(cs, f.Path)
			}
		}
		if dirs {
		d:
			for d := f.Path; d != ""; {
				d = path.Dir(d)
				if d == "." {
					d = ""
				}
				if _, ok := ds[d]; ok {
					continue d
				}
				ds[d] = struct{}{}
			}
		}
	}
	if dirs {
		for d := range ds {
			cs = append(cs, d+"/")
		}
	}

	slices.Sort(cs)
	cs = slices.Compact(cs)
	return cs, cobra.ShellCompDirectiveNoFileComp
}

// FlagIncludeExclude adds --exclude and --include flags, returning a function
// checking if a file is excluded.
func FlagIncludeExclude(out *func(tf2vpk.ValvePakFile) (bool, error), cmd *cobra.Command, short bool) {
	var Include, Exclude *[]string
	var (
		IncludeDoc = "Includes only files or directories matching one of the provided globs (matches everything if not specified)"
		ExcludeDoc = "Negates --include for files or directories matching the provided glob"
	)
	if short {
		Include = cmd.Flags().StringSliceP("include", "e", nil, IncludeDoc)
		Exclude = cmd.Flags().StringSliceP("exclude", "E", nil, ExcludeDoc)
	} else {
		Include = cmd.Flags().StringSlice("include", nil, IncludeDoc)
		Exclude = cmd.Flags().StringSlice("exclude", nil, ExcludeDoc)
	}
	*out = func(f tf2vpk.ValvePakFile) (bool, error) {
		included := len(*Include) == 0
		for _, x := range *Include {
			if m, err := internal.MatchGlobParents(x, f.Path); err != nil {
				return false, fmt.Errorf("process includes: match %q against glob %q: %w", f.Path, x, err)
			} else if m {
				included = true
				break
			}
		}
		for _, x := range *Exclude {
			if m, err := internal.MatchGlobParents(x, f.Path); err != nil {
				return false, fmt.Errorf("process excludes: match %q against glob %q: %w", f.Path, x, err)
			} else if m {
				included = false
				break
			}
		}
		return !included, nil
	}
}
