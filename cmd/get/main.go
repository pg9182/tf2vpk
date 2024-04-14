package get

import (
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/pg9182/tf2vpk"
	"github.com/pg9182/tf2vpk/cmd/root"
	"github.com/spf13/cobra"
)

var Flags struct {
	VPK   tf2vpk.ValvePakRef
	Files []string
}

var Command = &cobra.Command{
	GroupID: root.GroupVPKRead.ID,
	Use:     "get vpk_path file...",
	Aliases: []string{"cat"},
	Short:   "Reads files from a VPK to stdout",
	Args:    cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		Flags.Files = args[1:]
		main()
	},
}

func init() {
	root.ArgVPK(&Flags.VPK, Command, 1, true, false, true)
	root.Command.AddCommand(Command)
}

func main() {
	r, err := tf2vpk.NewReader(Flags.VPK)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open vpk: %v\n", err)
		os.Exit(1)
	}

	var failed int
	for _, name := range Flags.Files {
		if err := func() error {
			for _, f := range r.Root.File {
				if f.Path == name {
					r, err := r.OpenFileParallel(f, root.Flags.Threads)
					if err != nil {
						return err
					}
					if _, err := io.Copy(os.Stdout, r); err != nil {
						return err
					}
					return nil
				}
			}
			return fs.ErrNotExist
		}(); err != nil {
			fmt.Fprintf(os.Stderr, "error: read file %q: %v\n", name, err)
			failed++
		}
	}
	if failed != 0 {
		os.Exit(1)
	}
}
