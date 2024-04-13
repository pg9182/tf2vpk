// Command tf2-vpk2tar streams Titanfall 2 VPKs as a tar archive.
package main

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/pg9182/tf2vpk"
	"github.com/pg9182/tf2vpk/cmd/internal"
	"github.com/spf13/pflag"
)

var (
	Output    = pflag.StringP("output", "o", "-", "The file to write the tar archive to")
	VPKPrefix = pflag.StringP("vpk-prefix", "p", "english", "VPK prefix")
	Verbose   = pflag.BoolP("verbose", "v", false, "Write information about processed files to stderr")
	Test      = pflag.BoolP("test", "t", false, "Don't create a tar archive; only attempt to read the entire VPK and verify checksums")
	Threads   = pflag.IntP("threads", "j", runtime.NumCPU(), "The number of decompression threads to use (0 to only decompress chunks as they are read) (defaults to the number of cores)")

	Exclude = pflag.StringSlice("exclude", nil, "Excludes files or directories matching the provided glob (anchor to the start with /)")
	Include = pflag.StringSlice("include", nil, "Negates --exclude for files or directories matching the provided glob")

	Help = pflag.BoolP("help", "h", false, "Show this help message")
)

func main() {
	pflag.Parse()

	argv := pflag.Args()
	if len(argv) == 0 || len(argv) > 2 || *Help {
		fmt.Fprintf(os.Stderr, "usage: %s [options] (vpk_dir vpk_name)|vpk_path\n\noptions:\n%s", os.Args[0], pflag.CommandLine.FlagUsages())
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

	var r *tf2vpk.Reader
	var err error

	if len(argv) == 2 {
		vpkDir, vpkName := argv[0], argv[1]

		r, err = tf2vpk.OpenReader(vpkDir, *VPKPrefix, vpkName)
		if err != nil {
			err = fmt.Errorf("open vpk %q (prefix %q) from %q: %w", vpkName, *VPKPrefix, vpkDir, err)
		}
	} else {
		vpkPath := argv[0]

		r, err = tf2vpk.OpenReaderPath(vpkPath, *VPKPrefix)
		if err != nil {
			err = fmt.Errorf("open vpk %q (prefix %q): %w", vpkPath, *VPKPrefix, err)
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer r.Close()

	var w io.Writer
	if !*Test {
		switch *Output {
		case "":
			fmt.Fprintf(os.Stderr, "error: no output file specified\n")
			os.Exit(1)
		case "-":
			w = os.Stdout
		default:
			f, err := os.OpenFile(*Output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: create output file: %v\n", err)
				os.Exit(1)
			}
			defer f.Close()
			w = f
		}
	}

	var tw *tar.Writer
	if !*Test {
		tw = tar.NewWriter(w)
	}

	for _, f := range r.Root.File {
		var excluded bool
		for _, x := range *Exclude {
			if m, err := internal.MatchGlobParents(x, f.Path); err != nil {
				fmt.Fprintf(os.Stderr, "error: process excludes: match %q against glob %q: %v\n", f.Path, x, err)
				os.Exit(1)
			} else if m {
				excluded = true
			}
		}
		for _, x := range *Include {
			if m, err := internal.MatchGlobParents(x, f.Path); err != nil {
				fmt.Fprintf(os.Stderr, "error: process includes: match %q against glob %q: %v\n", f.Path, x, err)
				os.Exit(1)
			} else if m {
				excluded = false
			}
		}
		if excluded {
			continue
		}
		var sz uint64
		for _, c := range f.Chunk {
			sz += c.UncompressedSize
		}
		if *Verbose {
			fmt.Fprintf(os.Stderr, "%s\n", f.Path)
		}
		fr, err := r.OpenFileParallel(f, *Threads)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: read vpk file %q: %v\n", f.Path, err)
			os.Exit(1)
		}
		if *Test {
			_, err = io.Copy(io.Discard, fr)
		} else {
			if err = tw.WriteHeader(&tar.Header{
				Name:    f.Path,
				Size:    int64(sz),
				Mode:    0666,
				ModTime: time.Now(),
			}); err == nil {
				_, err = io.Copy(tw, fr)
			}
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: process vpk file %q: %v\n", f.Path, err)
			os.Exit(1)
		}
	}

	if !*Test {
		if err := tw.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "error: write output file %q: %v\n", *Output, err)
			os.Exit(1)
		}
	}

	if c, ok := w.(io.Closer); ok {
		if err := c.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "error: write output file %q: %v\n", *Output, err)
			os.Exit(1)
		}
	}
}
