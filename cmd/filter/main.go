package filter

import (
	"fmt"
	"os"
	"slices"

	"github.com/pg9182/tf2vpk"
	"github.com/pg9182/tf2vpk/cmd/root"
	"github.com/pg9182/tf2vpk/vpkutil"
	"github.com/spf13/cobra"
)

var Flags struct {
	VPK            tf2vpk.ValvePakRef
	IncludeExclude func(tf2vpk.ValvePakFile) (bool, error)
	Verbose        bool
	DryRun         bool
}

var Command = &cobra.Command{
	GroupID: root.GroupVPKWrite.ID,
	Use:     "filter vpk_path",
	Short:   "Filters files out of a VPK",
	Long: `Filters files out of a VPK

Does not remove the unused chunks; use the gc command to do that after filtering.
`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		main()
	},
}

func init() {
	root.ArgVPK(&Flags.VPK, Command, 2, true, true, true)
	root.FlagIncludeExclude(&Flags.IncludeExclude, Command, true)
	Command.Flags().BoolVarP(&Flags.DryRun, "dry-run", "n", false, "do not write changes")
	Command.Flags().BoolVarP(&Flags.Verbose, "verbose", "v", false, "print information about each filtered file")
	root.Command.AddCommand(Command)
}

func main() {
	var failed int
	if err := vpkutil.UpdateDir(Flags.VPK, Flags.DryRun, func(root *tf2vpk.ValvePakDir) error {
		var errs []error
		root.File = slices.DeleteFunc(root.File, func(f tf2vpk.ValvePakFile) bool {
			skip, err := Flags.IncludeExclude(f)
			if err != nil {
				errs = append(errs, err)
			}
			if skip && Flags.Verbose {
				fmt.Printf("filtered %s\n", f.Path)
			}
			return skip
		})
		if len(errs) != 0 {
			return errs[0]
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
