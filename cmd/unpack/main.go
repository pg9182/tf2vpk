package unpack

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pg9182/tf2vpk"
	"github.com/pg9182/tf2vpk/cmd/root"
	"github.com/pg9182/tf2vpk/internal"
	"github.com/pg9182/tf2vpk/vpkutil"
	"github.com/spf13/cobra"
)

var Flags struct {
	VPK              tf2vpk.ValvePakRef
	Path             string
	VPKFlagsExplicit bool
	VPKIgnoreEmpty   bool
	Verbose          bool
	IncludeExclude   func(tf2vpk.ValvePakFile) (bool, error)
}

var Command = &cobra.Command{
	GroupID: root.GroupVPKRepack.ID,
	Use:     "unpack vpk_path out_path",
	Short:   "Unpacks a VPK for modification and repacking",
	Args:    cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		Flags.Path = args[1]
		main()
	},
}

func init() {
	root.ArgVPK(&Flags.VPK, Command, -1, false, false, false)
	Command.Flags().BoolVarP(&Flags.VPKFlagsExplicit, "explicit-vpkflags", "x", false, "do not compute inherited vpkflags; generate one line for each file")
	Command.Flags().BoolVar(&Flags.VPKIgnoreEmpty, "empty-vpkignore", false, "do not add default vpkignore entires")
	Command.Flags().BoolVarP(&Flags.Verbose, "verbose", "v", false, "display progress information")
	root.FlagIncludeExclude(&Flags.IncludeExclude, Command, true)
	root.Command.AddCommand(Command)
}

func main() {
	if Flags.Verbose {
		fmt.Printf("unpacking vpk to %q\n", Flags.Path)
	}

	r, err := tf2vpk.NewReader(Flags.VPK)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open vpk: %v\n", err)
		os.Exit(1)
	}

	if Flags.Verbose {
		if Flags.VPKFlagsExplicit {
			fmt.Printf("... generating .vpkflags (without inheritance)\n")
		} else {
			fmt.Printf("... generating .vpkflags\n")
		}
	}
	var vpkflags vpkutil.VPKFlags
	if Flags.VPKFlagsExplicit {
		if err := vpkflags.GenerateExplicit(r.Root); err != nil {
			fmt.Fprintf(os.Stderr, "error: generate vpkflags without inheritance: %v\n", err)
			os.Exit(1)
		}
	} else {
		if err := vpkflags.Generate(r.Root); err != nil {
			fmt.Fprintf(os.Stderr, "error: generate vpkflags: %v\n", err)
			os.Exit(1)
		}
	}
	if err := vpkflags.Test(r.Root); err != nil {
		fmt.Println(vpkflags.String())
		panic(fmt.Errorf("BUG: test generated vpkflags: %w", err))
	}

	if Flags.Verbose {
		if Flags.VPKIgnoreEmpty {
			fmt.Printf("... generating .vpkignore (without default entries)\n")
		} else {
			fmt.Printf("... generating .vpkignore\n")
		}
	}
	var vpkignore vpkutil.VPKIgnore
	if !Flags.VPKIgnoreEmpty {
		vpkignore.AddDefault()
	}
	if err := vpkignore.AddAutoExclusions(r.Root); err != nil {
		fmt.Fprintf(os.Stderr, "error: generate vpkignore: %v\n", err)
		os.Exit(1)
	}

	if Flags.Verbose {
		fmt.Printf("... creating output directory\n")
	}
	if err := os.Mkdir(Flags.Path, 0777); err != nil && !errors.Is(err, fs.ErrExist) {
		fmt.Fprintf(os.Stderr, "error: create output directory: %v\n", err)
		os.Exit(1)
	}
	if dis, err := os.ReadDir(Flags.Path); err != nil {
		fmt.Fprintf(os.Stderr, "error: list output directory: %v\n", err)
		os.Exit(1)
	} else {
		for _, di := range dis {
			if !vpkignore.Match(di.Name()) {
				fmt.Fprintf(os.Stderr, "error: output directory must not exist or be empty (other than ignored files), found %q\n", di.Name())
				os.Exit(1)
			}
		}
	}

	if Flags.Verbose {
		fmt.Printf("... saving .vpkflags\n")
	}
	if err := os.WriteFile(filepath.Join(Flags.Path, ".vpkflags"), []byte(vpkflags.String()), 0666); err != nil {
		fmt.Fprintf(os.Stderr, "error: write .vpkflags: %v\n", err)
		os.Exit(1)
	}

	if Flags.Verbose {
		fmt.Printf("... saving .vpkignore\n")
	}
	if err := os.WriteFile(filepath.Join(Flags.Path, ".vpkignore"), []byte(vpkignore.String()), 0666); err != nil {
		fmt.Fprintf(os.Stderr, "error: write .vpkignore: %v\n", err)
		os.Exit(1)
	}

	if Flags.Verbose {
		fmt.Println()
	}
	var excludedCount int
	for i, f := range r.Root.File {
		if skip, err := Flags.IncludeExclude(f); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		} else if skip {
			excludedCount++
			if Flags.Verbose {
				fmt.Printf("[%4d/%4d] %s (excluded)\n", i+1, len(r.Root.File), f.Path)
			}
			continue
		}

		var uncompressed uint64
		for _, c := range f.Chunk {
			uncompressed += c.UncompressedSize
		}
		if Flags.Verbose {
			fmt.Printf("[%4d/%4d] %s (%s)\n", i+1, len(r.Root.File), f.Path, internal.FormatBytesSI(int64(uncompressed)))
		}

		outPath := filepath.Join(Flags.Path, filepath.FromSlash(f.Path))

		if err := os.MkdirAll(filepath.Dir(outPath), 0777); err != nil {
			fmt.Fprintf(os.Stderr, "error: create %q: %v\n", outPath, err)
			os.Exit(1)
		}

		tf, err := os.CreateTemp(Flags.Path, ".vpk*")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: create temp file: %v\n", err)
			os.Exit(1)
		}
		defer tf.Close()

		fr, err := r.OpenFileParallel(f, root.Flags.Threads)
		if err != nil {
			os.Remove(tf.Name())
			fmt.Fprintf(os.Stderr, "error: read vpk file %q: %v\n", f.Path, err)
			os.Exit(1)
		}

		if _, err := io.Copy(tf, fr); err != nil {
			os.Remove(tf.Name())
			fmt.Fprintf(os.Stderr, "error: extract vpk file %q: %v\n", f.Path, err)
			os.Exit(1)
		}

		if err := tf.Close(); err != nil {
			os.Remove(tf.Name())
			fmt.Fprintf(os.Stderr, "error: extract vpk file %q: %v\n", f.Path, err)
			os.Exit(1)
		}

		if err := os.Rename(tf.Name(), outPath); err != nil {
			fmt.Fprintf(os.Stderr, "error: extract vpk file %q: rename temp file: %v\n", f.Path, err)
			os.Exit(1)
		}

		// TODO: maybe extract files in parallel instead of using a parallel reader, might be faster for small files
	}
	if Flags.Verbose {
		if excludedCount != 0 {
			fmt.Printf("\nsuccess (%d files excluded by command-line filter)\n", excludedCount)
		} else {
			fmt.Printf("\nsuccess\n")
		}
	}
}
