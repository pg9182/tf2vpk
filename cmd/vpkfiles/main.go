package vpkfiles

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pg9182/tf2vpk"
	"github.com/pg9182/tf2vpk/cmd/root"
	"github.com/spf13/cobra"
)

var Command = &cobra.Command{
	GroupID: root.GroupVPKRepack.ID,
	Use:     "vpkfiles",
	Short:   "Manages files constituting a VPK",
}

var CommandList = subcommand(
	&cobra.Command{
		Use:     "list vpk_path",
		Short:   "List files constituting a VPK",
		Aliases: []string{"ls"},
		Args:    cobra.ExactArgs(1),
	},
	func(args []string, vpk tf2vpk.ValvePakRef, files []string, verbose, dryRun bool) error {
		for _, f := range files {
			fmt.Printf("%s\n", filepath.Join(vpk.Path, f))
		}
		return nil
	},
)

var CommandDelete = subcommand(
	&cobra.Command{
		Use:     "delete vpk_path",
		Short:   "Delete a VPK",
		Aliases: []string{"del", "rm"},
		Args:    cobra.ExactArgs(1),
	},
	func(args []string, vpk tf2vpk.ValvePakRef, files []string, verbose, dryRun bool) error {
		for _, f := range files {
			if verbose {
				fmt.Printf("delete %q\n", f)
			}
			if !dryRun {
				if err := os.Remove(f); err != nil && !errors.Is(err, fs.ErrNotExist) {
					return err
				}
			}
		}
		return nil
	},
)

var CommandRename = subcommand(
	&cobra.Command{
		Use:     "rename vpk_path new_name",
		Short:   "Change the name of a VPK",
		Aliases: []string{"ren"},
		Args:    cobra.ExactArgs(2),
	},
	func(args []string, vpk tf2vpk.ValvePakRef, files []string, verbose, dryRun bool) error {
		for _, f := range files {
			n, p := strings.CutPrefix(f, vpk.Prefix)
			n, ok := strings.CutPrefix(n, vpk.Name)
			if !ok {
				panic("wtf")
			}
			n = args[1] + n
			if p {
				n = vpk.Prefix + n
			}
			if f == n {
				continue
			}
			if verbose {
				fmt.Printf("rename %q -> %q\n", f, n)
			}
			if !dryRun {
				if err := os.Rename(filepath.Join(vpk.Path, f), filepath.Join(vpk.Path, n)); err != nil {
					return err
				}
			}
		}
		return nil
	},
)

var CommandPrefix = subcommand(
	&cobra.Command{
		Use:     "prefix vpk_path new_prefix",
		Short:   "Change the prefix of a VPK",
		Aliases: []string{"pfx", "pre"},
		Args:    cobra.ExactArgs(2),
	},
	func(args []string, vpk tf2vpk.ValvePakRef, files []string, verbose, dryRun bool) error {
		for _, f := range files {
			n, p := strings.CutPrefix(f, vpk.Prefix)
			if !p {
				continue
			}
			n = args[1] + n
			if f == n {
				continue
			}
			if verbose {
				fmt.Printf("rename %q -> %q\n", f, n)
			}
			if !dryRun {
				if err := os.Rename(filepath.Join(vpk.Path, f), filepath.Join(vpk.Path, n)); err != nil {
					return err
				}
			}
		}
		return nil
	},
)

func subcommand(cmd *cobra.Command, fn func(args []string, vpk tf2vpk.ValvePakRef, files []string, verbose, dryRun bool) error) *cobra.Command {
	var flags struct {
		VPK     tf2vpk.ValvePakRef
		Verbose bool
		DryRun  bool
	}
	cmd.Run = func(cmd *cobra.Command, args []string) {
		files, err := flags.VPK.List()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: list vpk files: %v\n", err)
			os.Exit(1)
		}
		// deterministically sort dir first, then paks alphabetically
		slices.SortStableFunc(files, func(a, b string) int {
			ax := strings.HasPrefix(a, flags.VPK.Prefix)
			bx := strings.HasPrefix(b, flags.VPK.Prefix)
			if ax != bx {
				if ax {
					return -1
				}
				if bx {
					return 1
				}
				return 0
			}
			return strings.Compare(a, b)
		})
		if err := fn(args, flags.VPK, files, flags.Verbose, flags.DryRun); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
	if cmd.Use != "list vpk_path" {
		cmd.Flags().BoolVarP(&flags.Verbose, "verbose", "v", false, "log file operations")
		cmd.Flags().BoolVarP(&flags.DryRun, "dry-run", "n", false, "don't actually do anything")
	}
	root.ArgVPK(&flags.VPK, cmd, -1, false, false, false)
	return cmd
}

func init() {
	Command.AddCommand(CommandList)
	Command.AddCommand(CommandDelete)
	Command.AddCommand(CommandRename)
	Command.AddCommand(CommandPrefix)
	root.Command.AddCommand(Command)
}
