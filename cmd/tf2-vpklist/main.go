// Command tf2-vpklist shows information about a VPK.
package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/pg9182/tf2vpk"
	"github.com/pg9182/tf2vpk/internal"
	"github.com/pg9182/tf2vpk/vpkutil"
	"github.com/spf13/pflag"
)

var (
	ResolveOpen = vpkutil.NewCLIResolveOpen(pflag.CommandLine, 0, false)

	HumanReadable      = pflag.BoolP("human-readable", "h", false, "Show values in human-readable form")
	HumanReadableFlags = pflag.BoolP("human-readable-flags", "f", false, "If displaying flags, also show them in human-readable form at the very end of the line (delimited by a #)")
	Long               = pflag.BoolP("long", "l", false, "Show detailed file metadata (adds the following columns to the beginning: block_index load_flags[binary] texture_flags[binary] crc32[hex] compressed_size[bytes] uncompressed_size[bytes] compressed_percent)")
	Test               = pflag.BoolP("test", "t", false, "Also attempt to read contents and compute checksums (adds a column with OK/ERR to the end)")
	//Stats = pflag.BoolP("stats", "s", false, "Show detailed statistics about vpk space utilization")

	Threads = pflag.IntP("threads", "j", runtime.NumCPU(), "The number of decompression threads to use while verifying checksums (0 to only decompress chunks as they are read) (defaults to the number of cores)")

	IncludeExclude = vpkutil.NewCLIIncludeExclude(pflag.CommandLine)

	Help = pflag.Bool("help", false, "Show this help message")
)

func main() {
	pflag.Parse()

	if !ResolveOpen.ArgCheck() || *Help {
		fmt.Fprintf(os.Stderr, "usage: %s [options] %s\n\noptions:\n%s", os.Args[0], ResolveOpen.ArgHelp(), pflag.CommandLine.FlagUsages())
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

	_, r, err := ResolveOpen.ResolveOpen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer r.Close()

	var pathLen int
	for _, f := range r.Root.File {
		pathLen = max(pathLen, min(len(f.Path), 64))
	}

	var testErrCount int
	for _, f := range r.Root.File {
		if skip, err := IncludeExclude.Skip(f); err != nil {
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

		if *Long {
			if *HumanReadable {
				fmt.Printf("%s %032b %016b %08X %6.2f %% %9s %9s  ", f.Index, load, texture, f.CRC32, float64(compressed)/float64(uncompressed)*100, formatBytesSIAligned(int64(compressed)), formatBytesSIAligned(int64(uncompressed)))
			} else {
				fmt.Printf("%s %032b %016b %08X %6.2f %% %9d %9d  ", f.Index, load, texture, f.CRC32, float64(compressed)/float64(uncompressed)*100, compressed, uncompressed)
			}
		}
		if *Test || (*Long && *HumanReadableFlags) {
			fmt.Printf("%*s", -pathLen, f.Path)
		} else {
			fmt.Printf("%s", f.Path)
		}
		if *Test {
			os.Stdout.Sync()
		}

		var testErr error
		if *Test {
			if fr, err := r.OpenFileParallel(f, *Threads); err != nil {
				testErr = err
			} else if _, err = io.Copy(io.Discard, fr); err != nil {
				testErr = err
			}
			if testErr != nil {
				testErrCount++
			}
		}

		if *Test {
			if testErr != nil {
				fmt.Printf(" ERR")
			} else {
				fmt.Printf("  OK")
			}
		}
		if *Long && *HumanReadableFlags {
			fmt.Printf(" # load=%s texture=%s", tf2vpk.DescribeLoadFlags(load), tf2vpk.DescribeTextureFlags(texture))
		}
		fmt.Printf("\n")

		if *Test && testErr != nil {
			fmt.Fprintf(os.Stderr, "warning: entry %q: test: %v\n", f.Path, testErr)
		}
	}
	// if *Stats {
	// }
	if *Test {
		fmt.Fprintf(os.Stderr, "%d/%d files valid", testErrCount, len(r.Root.File))
		if testErrCount != 0 {
			os.Exit(1)
		}
	}
}

// TODO: stats, something like:
//     blocks (#)                 #/# MB [%] uncompressed
//
//       001                      #/# MB [%] uncompressed
//         chunks
//           unused               #/# MB [%] uncompressed
//           used (#)             #/# MB [%] uncompressed
//             by compression
//               compressed (#)   #/# MB [%] uncompressed
//               uncompressed (#) # MB
//             by reuse
//               2 files (#)      #/# MB [%] uncompressed
//               1 file (#)       #/# MB [%] uncompressed
//         files
//           files (#)            #/# MB [%] uncompressed
//             by top dir
//               dir (#)          #/# MB [%] uncompressed
//             by extension
//               ext (#)          #/# MB [%] uncompressed
//
//       all                      #/# MB [%] uncompressed
//         chunks
//           unused               #/# MB [%] uncompressed
//           used (#)             #/# MB [%] uncompressed
//             by compression
//               compressed (#)   #/# MB [%] uncompressed
//               uncompressed (#) # MB
//             by reuse
//               2 files (#)      #/# MB [%] uncompressed
//               1 file (#)       #/# MB [%] uncompressed
//         files
//           files (#)            #/# MB [%] uncompressed
//             by top dir
//               dir (#)          #/# MB [%] uncompressed
//             by extension
//               ext (#)          #/# MB [%] uncompressed
//
//     vpk index                    # MB

func formatBytesSIAligned(b int64) string {
	s := internal.FormatBytesSI(b)
	s, isB := strings.CutSuffix(s, " B")
	if isB {
		s += "  B"
	}
	return s
}
