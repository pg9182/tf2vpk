package chflg

import (
	"encoding/binary"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"

	"github.com/pg9182/tf2vpk"
	"github.com/pg9182/tf2vpk/cmd/root"
	"github.com/pg9182/tf2vpk/vpkutil"
	"github.com/spf13/cobra"
)

var Flags struct {
	VPK     tf2vpk.ValvePakRef
	Flags   string
	Files   []string
	Verbose bool
	DryRun  bool
}

var Command = &cobra.Command{
	GroupID: root.GroupVPKWrite.ID,
	Use:     "chflg vpk_path { load_flags:texture_flags | @reference_file } file...",
	Aliases: []string{"chflag", "chflags"},
	Short:   "Sets flags for VPK entries",
	Long: `Sets flags for VPK entries

Flags are specified as a fixed-width bit string, as hex prefixed with 0x, or as a path to another in the VPK to copy flags from.

The provided file can also be a directory to change all files under it (use / to change everything).
`,
	Args: cobra.MinimumNArgs(3),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 1 {
			if toComplete, ok := strings.CutPrefix(toComplete, "@"); ok {
				cs, rc := root.ArgVPKFileCompletions(args, toComplete, false, true)
				for i := range cs {
					cs[i] = "@" + cs[i]
				}
				return cs, rc
			}
		}
		return nil, cobra.ShellCompDirectiveDefault
	},
	Run: func(cmd *cobra.Command, args []string) {
		Flags.Flags = args[1]
		Flags.Files = args[2:]
		main()
	},
}

func init() {
	root.ArgVPK(&Flags.VPK, Command, 2, true, true, true)
	Command.Flags().BoolVarP(&Flags.DryRun, "dry-run", "n", false, "do not write changes")
	Command.Flags().BoolVarP(&Flags.Verbose, "verbose", "v", false, "print information about each processed file")
	root.Command.AddCommand(Command)
}

func main() {
	var failed int
	if err := vpkutil.UpdateDir(Flags.VPK, Flags.DryRun, func(root *tf2vpk.ValvePakDir) error {
		var (
			err          error
			loadFlags    uint32
			textureFlags uint16
		)
		if p, ok := strings.CutPrefix(Flags.Flags, "@"); ok {
			var found bool
			for _, f := range root.File {
				if f.Path == strings.TrimPrefix(p, "/") {
					if loadFlags, err = f.LoadFlags(); err != nil {
						fmt.Fprintf(os.Stderr, "error: failed to compute load flags for reference file %q: %v\n", p, err)
						os.Exit(1)
					}
					if textureFlags, err = f.TextureFlags(); err != nil {
						fmt.Fprintf(os.Stderr, "error: failed to compute texture flags for reference file %q: %v\n", p, err)
						os.Exit(1)
					}
					found = true
					break
				}
			}
			if !found {
				fmt.Fprintf(os.Stderr, "error: reference file %q does not exist in vpk\n", p)
				os.Exit(1)
			}
		} else {
			loadFlags, textureFlags, err = parseFlags(Flags.Flags)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: invalid flags %q: %v\n", Flags.Flags, err)
				os.Exit(1)
			}
		}

		for _, name := range Flags.Files {
			if err := func() error {
				var matched bool
				for i, f := range root.File {
					if name == "/" || strings.HasPrefix(f.Path+"/", name+"/") {
						loadFlagsOrig, _ := f.LoadFlags()
						textureFlagsOrig, _ := f.TextureFlags()
						for j := range f.Chunk {
							root.File[i].Chunk[j].LoadFlags = loadFlags
							root.File[i].Chunk[j].TextureFlags = textureFlags
						}
						if Flags.Verbose {
							var what string
							if loadFlagsOrig != loadFlags || textureFlagsOrig != textureFlags {
								what = "set flags to"
							} else {
								what = "retained flags"
							}
							fmt.Printf("%s: %s 0x%08X:0x%04X (load=%s texture=%s)\n", f.Path, what, loadFlags, textureFlags, tf2vpk.DescribeLoadFlags(loadFlags), tf2vpk.DescribeTextureFlags(textureFlags))
						}
						matched = true
					}
				}
				if !matched {
					return fs.ErrNotExist
				}
				return nil
			}(); err != nil {
				fmt.Fprintf(os.Stderr, "error: set flags for file %q: %v\n", name, err)
				failed++
			}
		}

		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if failed != 0 {
		os.Exit(1)
	}
}

func parseFlags(s string) (uint32, uint16, error) {
	load, texture, ok := strings.Cut(s, ":")
	if !ok {
		return 0, 0, fmt.Errorf("expected load and texture flags separated by a colon")
	}
	loadFlags, err := parseFlag[uint32](load)
	if err != nil {
		return 0, 0, fmt.Errorf("parse load flags: %v", err)
	}
	textureFlags, err := parseFlag[uint16](texture)
	if err != nil {
		return 0, 0, fmt.Errorf("parse texture flags: %v", err)
	}
	return loadFlags, textureFlags, nil
}

func parseFlag[T uint16 | uint32](s string) (T, error) {
	bits := binary.Size(T(0)) * 8
	if s, ok := strings.CutPrefix(s, "0x"); ok {
		if v, err := strconv.ParseUint(s, 16, bits); err != nil {
			return 0, fmt.Errorf("parse hex flags %q: %w", s, err)
		} else {
			return T(v), nil
		}
	}
	if v, err := strconv.ParseUint(s, 2, bits); err == nil {
		if len(s) != bits {
			return 0, fmt.Errorf("parse binary flags %q: must be exactly %d bits", s, bits)
		} else {
			return T(v), nil
		}
	}
	return 0, fmt.Errorf("unknown flag format %q (expected 0x hex or fixed-width binary)", s)
}
