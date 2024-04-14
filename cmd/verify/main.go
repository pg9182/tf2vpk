package verify

import (
	"fmt"
	"io"
	"os"

	"github.com/pg9182/tf2vpk"
	"github.com/pg9182/tf2vpk/cmd/root"
	"github.com/spf13/cobra"
)

var Flags struct {
	VPK     tf2vpk.ValvePakRef
	Verbose bool
}

var Command = &cobra.Command{
	GroupID: root.GroupVPKRead.ID,
	Use:     "verify vpk_path",
	Short:   "Verifies the contents of a VPK",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		main()
	},
}

func init() {
	root.ArgVPK(&Flags.VPK, Command, -1, false, false, false)
	Command.Flags().BoolVarP(&Flags.Verbose, "verbose", "v", false, "display files as they are verified")
	root.Command.AddCommand(Command)
}

func main() {
	r, err := tf2vpk.NewReader(Flags.VPK)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open vpk: %v\n", err)
		os.Exit(1)
	}

	var failure int
	for _, f := range r.Root.File {
		if Flags.Verbose {
			fmt.Printf("%s: ", f.Path)
			os.Stderr.Sync()
		}
		if err := func() error {
			r, err := r.OpenFileParallel(f, root.Flags.Threads)
			if err != nil {
				return err
			}
			if _, err := io.Copy(io.Discard, r); err != nil {
				return err
			}
			return nil
		}(); err != nil {
			if Flags.Verbose {
				fmt.Printf("ERROR\n")
			}
			fmt.Fprintf(os.Stderr, "%s: ERROR - %v\n", f.Path, err)
			failure++
		} else {
			if Flags.Verbose {
				fmt.Printf("OK\n")
			}
		}
	}
	if failure != 0 {
		os.Exit(1)
	}
}
