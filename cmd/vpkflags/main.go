package vpkflags

import (
	"fmt"
	"os"

	"github.com/pg9182/tf2vpk"
	"github.com/pg9182/tf2vpk/cmd/root"
	"github.com/pg9182/tf2vpk/vpkutil"
	"github.com/spf13/cobra"
)

var Flags struct {
	VPK      tf2vpk.ValvePakRef
	Explicit bool
}

var Command = &cobra.Command{
	GroupID: root.GroupVPKRepack.ID,
	Use:     "vpkflags vpk_path",
	Short:   "Generates a vpkflags file based on an existing VPK",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		main()
	},
}

func init() {
	root.ArgVPK(&Flags.VPK, Command, -1, false, false, false)
	Command.Flags().BoolVarP(&Flags.Explicit, "explicit", "x", false, "do not compute inherited vpkflags; generate one line for each file")
	root.Command.AddCommand(Command)
}

func main() {
	r, err := tf2vpk.NewReader(Flags.VPK)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open vpk: %v\n", err)
		os.Exit(1)
	}

	var vpkflags vpkutil.VPKFlags
	if Flags.Explicit {
		err = vpkflags.GenerateExplicit(r.Root)
	} else {
		err = vpkflags.Generate(r.Root)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: generate vpkflags: %v\n", err)
		os.Exit(1)
	}
	if _, err := os.Stdout.WriteString(vpkflags.String()); err != nil {
		os.Exit(1)
	}
}
