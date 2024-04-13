// Command tf2-vpkunpack unpacks or initializes a new vpk for modification and
// repacking.
package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"github.com/pg9182/tf2vpk/internal"
	"github.com/pg9182/tf2vpk/vpkutil"
	"github.com/spf13/pflag"
)

var (
	ResolveOpen = vpkutil.NewCLIResolveOpen(pflag.CommandLine, 1, true)

	VPKFlagsExplicit = pflag.Bool("vpkflags-explicit", false, "Do not optimize vpkflags for inheritance; generate one line for each file")
	VPKIgnoreEmpty   = pflag.Bool("vpkignore-no-default", false, "Do not add default vpkignore entries")
	Threads          = pflag.IntP("threads", "j", runtime.NumCPU(), "The number of decompression threads to use while verifying checksums (0 to only decompress chunks as they are read) (defaults to the number of cores)")

	IncludeExclude = vpkutil.NewCLIIncludeExclude(pflag.CommandLine)

	Help = pflag.Bool("help", false, "Show this help message")
)

func main() {
	pflag.Parse()

	args := pflag.Args()
	if len(args) == 0 || !ResolveOpen.ArgCheck() || *Help {
		fmt.Fprintf(os.Stderr, "usage: %s [options] empty_output_path %s\n\noptions:\n%s", os.Args[0], ResolveOpen.ArgHelp(), pflag.CommandLine.FlagUsages())
		if !*Help {
			os.Exit(2)
		}
		return
	}

	if *Threads < 0 {
		*Threads = 0
	}
	if *Threads > runtime.NumCPU() {
		runtime.GOMAXPROCS(*Threads)
	}

	vpkOut := args[0]

	if len(args) == 1 {
		fmt.Printf("initializing new vpk in %q\n", vpkOut)
	} else {
		fmt.Printf("unpacking vpk to %q\n", vpkOut)
	}

	_, r, err := ResolveOpen.ResolveOpen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if *VPKFlagsExplicit && r != nil {
		fmt.Printf("... generating .vpkflags (without inheritance)\n")
	} else {
		fmt.Printf("... generating .vpkflags\n")
	}
	var vpkFlags vpkutil.VPKFlags
	if r != nil {
		if *VPKFlagsExplicit {
			if err := vpkFlags.GenerateExplicit(r.Root); err != nil {
				fmt.Fprintf(os.Stderr, "error: generate vpkflags without inheritance: %v\n", err)
				os.Exit(1)
			}
		} else {
			if err := vpkFlags.Generate(r.Root); err != nil {
				fmt.Fprintf(os.Stderr, "error: generate vpkflags: %v\n", err)
				os.Exit(1)
			}
		}
		if err := vpkFlags.Test(r.Root); err != nil {
			fmt.Println(vpkFlags.String())
			panic(fmt.Errorf("BUG: test generated vpkflags: %w", err))
		}
	}

	if *VPKIgnoreEmpty {
		fmt.Printf("... generating .vpkignore (without default entries)\n")
	} else {
		fmt.Printf("... generating .vpkignore\n")
	}
	var vpkIgnore vpkutil.VPKIgnore
	if !*VPKIgnoreEmpty {
		vpkIgnore.AddDefault()
	}
	if r != nil {
		if err := vpkIgnore.AddAutoExclusions(r.Root); err != nil {
			fmt.Fprintf(os.Stderr, "error: generate vpkignore: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("... creating output directory\n")
	if err := os.Mkdir(vpkOut, 0777); err != nil && !errors.Is(err, fs.ErrExist) {
		fmt.Fprintf(os.Stderr, "error: create output directory: %v\n", err)
		os.Exit(1)
	}
	if dis, err := os.ReadDir(vpkOut); err != nil {
		fmt.Fprintf(os.Stderr, "error: list output directory: %v\n", err)
		os.Exit(1)
	} else {
		for _, di := range dis {
			if !vpkIgnore.Match(di.Name()) {
				fmt.Fprintf(os.Stderr, "error: output directory must not exist or be empty (other than ignored files), found %q\n", di.Name())
				os.Exit(1)
			}
		}
	}

	fmt.Printf("... saving .vpkflags\n")
	if err := os.WriteFile(filepath.Join(vpkOut, ".vpkflags"), []byte(vpkFlags.String()), 0666); err != nil {
		fmt.Fprintf(os.Stderr, "error: write .vpkflags: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("... saving .vpkignore\n")
	if err := os.WriteFile(filepath.Join(vpkOut, ".vpkignore"), []byte(vpkIgnore.String()), 0666); err != nil {
		fmt.Fprintf(os.Stderr, "error: write .vpkignore: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()

	var excludedCount int
	if r != nil {
		for i, f := range r.Root.File {
			if skip, err := IncludeExclude.Skip(f); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			} else if skip {
				excludedCount++
				fmt.Printf("[%4d/%4d] %s (excluded)\n", i+1, len(r.Root.File), f.Path)
				continue
			}

			var uncompressed uint64
			for _, c := range f.Chunk {
				uncompressed += c.UncompressedSize
			}
			fmt.Printf("[%4d/%4d] %s (%s)\n", i+1, len(r.Root.File), f.Path, internal.FormatBytesSI(int64(uncompressed)))

			outPath := filepath.Join(vpkOut, filepath.FromSlash(f.Path))

			if err := os.MkdirAll(filepath.Dir(outPath), 0777); err != nil {
				fmt.Fprintf(os.Stderr, "error: create %q: %v\n", outPath, err)
				os.Exit(1)
			}

			tf, err := os.CreateTemp(vpkOut, ".vpk*")
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: create temp file: %v\n", err)
				os.Exit(1)
			}
			defer tf.Close()

			fr, err := r.OpenFileParallel(f, *Threads)
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
	}

	fmt.Println()

	if excludedCount != 0 {
		fmt.Printf("success (%d files excluded by command-line filter)\n", excludedCount)
	} else {
		fmt.Printf("success\n")
	}
}
