package init

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pg9182/tf2vpk/cmd/root"
	"github.com/pg9182/tf2vpk/vpkutil"
	"github.com/spf13/cobra"
)

var Flags struct {
	Path  string
	Force bool
}

var Command = &cobra.Command{
	GroupID: root.GroupVPKRepack.ID,
	Use:     "init [out_path]",
	Short:   "Initializes vpkflags and vpkignore so a directory can be packed",
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			Flags.Path = "."
		} else {
			Flags.Path = args[0]
		}
		main()
	},
}

func init() {
	Command.Flags().BoolVarP(&Flags.Force, "force", "f", false, "overwrite files if they exist")
	root.Command.AddCommand(Command)
}

func main() {
	if err := os.Mkdir(Flags.Path, 0777); err != nil && !errors.Is(err, fs.ErrExist) {
		fmt.Fprintf(os.Stderr, "error: create output directory: %v\n", err)
		os.Exit(1)
	}

	writeFile := writeFileExcl
	if Flags.Force {
		writeFile = os.WriteFile
	}

	var vpkflags vpkutil.VPKFlags
	if err := vpkflags.Add("/", 0b00000000000000000000000100000001, 0); err != nil {
		panic(err)
	}

	var vpkignore vpkutil.VPKIgnore
	vpkignore.AddDefault()

	var fail bool
	if err := writeFile(filepath.Join(Flags.Path, vpkutil.VPKFlagsFilename), []byte(vpkflags.String()), 0666); err != nil {
		fmt.Fprintf(os.Stderr, "error: save vpkflags: %v\n", err)
		fail = true
	}
	if err := writeFile(filepath.Join(Flags.Path, vpkutil.VPKIgnoreFilename), []byte(vpkignore.String()), 0666); err != nil {
		fmt.Fprintf(os.Stderr, "error: save vpkignore: %v\n", err)
		fail = true
	}
	if fail {
		os.Exit(1)
	}
}

func writeFileExcl(name string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err != nil {
		os.Remove(name)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}
