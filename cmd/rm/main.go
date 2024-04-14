package rm

import (
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"

	"github.com/pg9182/tf2vpk"
	"github.com/pg9182/tf2vpk/cmd/root"
	"github.com/pg9182/tf2vpk/vpkutil"
	"github.com/spf13/cobra"
)

var Flags struct {
	VPK     tf2vpk.ValvePakRef
	Files   []string
	Force   bool
	Verbose bool
	DryRun  bool
}

var Command = &cobra.Command{
	GroupID: root.GroupVPKWrite.ID,
	Use:     "rm vpk_path file...",
	Aliases: []string{"remove", "delete", "del"},
	Short:   "Delete files or directories from a VPK",
	Long: `Delete files or directories from a VPK

Does not remove the unused chunks; use the gc command to do that afterwards.
`,
	Args: cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		Flags.Files = args[1:]
		main()
	},
}

func init() {
	root.ArgVPK(&Flags.VPK, Command, 1, true, true, true)
	Command.Flags().BoolVarP(&Flags.Force, "force", "f", false, "ignore non-existent files")
	Command.Flags().BoolVarP(&Flags.DryRun, "dry-run", "n", false, "do not write changes")
	Command.Flags().BoolVarP(&Flags.Verbose, "verbose", "v", false, "print information about each processed file")
	root.Command.AddCommand(Command)
}

func main() {
	var failed int
	if err := vpkutil.UpdateDir(Flags.VPK, Flags.DryRun, func(root *tf2vpk.ValvePakDir) error {
		for _, name := range Flags.Files {
			if err := func() error {
				orig := len(root.File)
				root.File = slices.DeleteFunc(root.File, func(f tf2vpk.ValvePakFile) bool {
					match := name == "/" || strings.HasPrefix(f.Path+"/", name+"/")
					if match {
						if Flags.Verbose {
							fmt.Printf("delete %s\n", f.Path)
						}
					}
					return match
				})
				if orig == len(root.File) && !Flags.Force {
					return fs.ErrNotExist
				}
				return nil
			}(); err != nil {
				fmt.Fprintf(os.Stderr, "error: delete %q: %v\n", name, err)
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
