package list

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pg9182/tf2vpk"
	"github.com/pg9182/tf2vpk/cmd/root"
	"github.com/pg9182/tf2vpk/internal"
	"github.com/spf13/cobra"
)

var Flags struct {
	VPK                tf2vpk.ValvePakRef
	HumanReadable      bool
	HumanReadableFlags bool
	Long               bool
	Test               bool
	IncludeExclude     func(tf2vpk.ValvePakFile) (bool, error)
}

var Command = &cobra.Command{
	GroupID: root.GroupVPKRead.ID,
	Use:     "list vpk_path",
	Short:   "Lists the contents of a VPK",
	Args:    cobra.ExactArgs(1),
	Aliases: []string{"ls"},
	Run: func(cmd *cobra.Command, args []string) {
		main()
	},
}

func init() {
	Command.Flags().Bool("help", false, "help for "+Command.Name()) // prevent the default short help flag from being set
	Command.Flags().BoolVarP(&Flags.HumanReadable, "human-readable", "h", false, "show values in human-readable form")
	Command.Flags().BoolVarP(&Flags.HumanReadableFlags, "human-readable-flags", "f", false, "if displaying flags, also show them in human-readable form at the very end of the line (delimited by a #)")
	Command.Flags().BoolVarP(&Flags.Long, "long", "l", false, "show detailed file metadata (adds the following columns to the beginning: block_index load_flags[binary] texture_flags[binary] crc32[hex] compressed_size[bytes] uncompressed_size[bytes] compressed_percent)")
	Command.Flags().BoolVarP(&Flags.Test, "test", "t", false, "also attempt to read contents and compute checksums (adds a column with OK/ERR to the end)")
	root.FlagIncludeExclude(&Flags.IncludeExclude, Command, true)
	root.ArgVPK(&Flags.VPK, Command, -1, false, false, false)
	root.Command.AddCommand(Command)
}

func main() {
	r, err := tf2vpk.NewReader(Flags.VPK)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open vpk: %v\n", err)
		os.Exit(1)
	}

	var pathLen int
	for _, f := range r.Root.File {
		pathLen = max(pathLen, min(len(f.Path), 64))
	}

	var testErrCount int
	for _, f := range r.Root.File {
		if skip, err := Flags.IncludeExclude(f); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		} else if skip {
			continue
		}

		load, err := f.LoadFlags()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: entry %q: compute load flags: %v\n", f.Path, err)
			load = 0
			load--
		}
		texture, err := f.TextureFlags()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: entry %q: compute texture flags: %v\n", f.Path, err)
			load = 0
			load--
		}

		var compressed, uncompressed uint64
		for _, c := range f.Chunk {
			compressed += c.CompressedSize
			uncompressed += c.UncompressedSize
		}

		if Flags.Long {
			if Flags.HumanReadable {
				fmt.Printf("%s %032b %016b %08X %6.2f %% %9s %9s  ", f.Index, load, texture, f.CRC32, float64(compressed)/float64(uncompressed)*100, formatBytesSIAligned(int64(compressed)), formatBytesSIAligned(int64(uncompressed)))
			} else {
				fmt.Printf("%s %032b %016b %08X %6.2f %% %9d %9d  ", f.Index, load, texture, f.CRC32, float64(compressed)/float64(uncompressed)*100, compressed, uncompressed)
			}
		}
		if Flags.Test || (Flags.Long && Flags.HumanReadableFlags) {
			fmt.Printf("%*s", -pathLen, f.Path)
		} else {
			fmt.Printf("%s", f.Path)
		}
		if Flags.Test {
			os.Stdout.Sync()
		}

		var testErr error
		if Flags.Test {
			if fr, err := r.OpenFileParallel(f, root.Flags.Threads); err != nil {
				testErr = err
			} else if _, err = io.Copy(io.Discard, fr); err != nil {
				testErr = err
			}
			if testErr != nil {
				testErrCount++
			}
		}

		if Flags.Test {
			if testErr != nil {
				fmt.Printf(" ERR")
			} else {
				fmt.Printf("  OK")
			}
		}
		if Flags.Long && Flags.HumanReadableFlags {
			fmt.Printf(" # load=%s texture=%s", tf2vpk.DescribeLoadFlags(load), tf2vpk.DescribeTextureFlags(texture))
		}
		fmt.Printf("\n")

		if Flags.Test && testErr != nil {
			fmt.Fprintf(os.Stderr, "warning: entry %q: test: %v\n", f.Path, testErr)
		}
	}
	if Flags.Test {
		fmt.Fprintf(os.Stderr, "%d/%d files valid", testErrCount, len(r.Root.File))
		if testErrCount != 0 {
			os.Exit(1)
		}
	}
}

func formatBytesSIAligned(b int64) string {
	s := internal.FormatBytesSI(b)
	s, isB := strings.CutSuffix(s, " B")
	if isB {
		s += "  B"
	}
	return s
}
