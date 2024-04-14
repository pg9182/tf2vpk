package lzham

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/pg9182/tf2lzham"
	"github.com/pg9182/tf2vpk/cmd/root"
	"github.com/spf13/cobra"
)

const ext = ".lzham"

var Flags struct {
	Files      []string
	Stdout     bool
	Decompress bool
	Keep       bool
	Force      bool
	Verbose    bool
	Buffer     uint
}

var Command = &cobra.Command{
	Use:   "lzham [file...]",
	Short: "Compresses or uncompress lzham files",
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if Flags.Decompress {
			return []string{strings.TrimPrefix(ext, ".")}, cobra.ShellCompDirectiveFilterFileExt
		}
		return nil, cobra.ShellCompDirectiveDefault
	},
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			Flags.Files = []string{"-"}
		} else {
			Flags.Files = args
		}
		main()
	},
}

func init() {
	Command.Flags().BoolVarP(&Flags.Stdout, "stdout", "c", false, "write to stdout, keep original file unchanged (always enabled if reading from stdin)")
	Command.Flags().BoolVarP(&Flags.Decompress, "decompress", "d", false, "decompress")
	Command.Flags().BoolVarP(&Flags.Keep, "keep", "k", false, "keep (don't delete) input files (always enabled if writing to stdout)")
	Command.Flags().BoolVarP(&Flags.Force, "force", "f", false, "force overwrite of output file")
	Command.Flags().BoolVarP(&Flags.Verbose, "verbose", "v", false, "verbose mode")
	Command.Flags().UintVarP(&Flags.Buffer, "buffer", "b", 5*1024*1024, "compression/decompression buffer size")
	root.Command.AddCommand(Command)
}

func main() {
	zbuf := make([]byte, int(Flags.Buffer))

	var failed int
	for _, input := range Flags.Files {
		if err := func() error {
			var output string
			if input == "-" || Flags.Stdout {
				output = "stdout"
			} else if Flags.Decompress {
				var ok bool
				if output, ok = strings.CutSuffix(input, ext); !ok {
					return fmt.Errorf("unknown extension (expected %s), ignoring", ext)
				}
			} else {
				output = input + ext
			}

			var mode fs.FileMode
			if s, err := os.Stat(input); err == nil {
				mode = s.Mode()
			} else {
				mode = 0666
			}

			var (
				buf []byte
				err error
			)
			if input == "-" {
				buf, err = io.ReadAll(os.Stdin)
			} else {
				buf, err = os.ReadFile(input)
			}
			if err != nil {
				return err
			}
			if len(buf) == 0 {
				return fmt.Errorf("input is empty")
			}

			var (
				n       int
				adler32 uint32
				crc32   uint32
			)
			if Flags.Decompress {
				n, adler32, crc32, err = tf2lzham.Decompress(zbuf, buf)
			} else {
				n, adler32, crc32, err = tf2lzham.Compress(zbuf, buf)
			}
			if err != nil {
				return err
			}

			if input == "-" || Flags.Stdout {
				if _, err := os.Stdout.Write(zbuf[:n]); err != nil {
					return err
				}
			} else {
				f, err := os.OpenFile(output, os.O_CREATE|os.O_TRUNC|os.O_WRONLY|os.O_EXCL, mode)
				if errors.Is(err, fs.ErrExist) {
					if !Flags.Force {
						fmt.Fprintf(os.Stderr, "warning: %s already exists; overwrite (y or n)? ", output)
						os.Stderr.Sync()

						var s string
						_, _ = fmt.Fscanln(os.Stdin, &s)

						if strings.TrimSpace(s) != "y" {
							fmt.Fprintf(os.Stderr, "\tnot overwriting\n")
							failed++
							return nil
						}
					}
					f, err = os.OpenFile(output, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
				}
				if err != nil {
					return err
				}
				defer f.Close()

				if _, err := f.Write(zbuf[:n]); err != nil {
					return err
				}
				if err := f.Close(); err != nil {
					return err
				}
			}

			var action string
			if !(Flags.Keep || input == "-" || Flags.Stdout) {
				if err := os.Remove(input); err != nil && !errors.Is(err, fs.ErrNotExist) {
					return err
				}
				action = "replaced with"
			} else {
				action = "created"
			}

			if Flags.Verbose {
				fmt.Fprintf(os.Stderr, "%s: %5.1f%% adler32=%08X crc32=%08X -- %s %s\n", input, float64(n)/float64(len(buf))*100, adler32, crc32, action, output)
			}
			return nil
		}(); err != nil {
			failed++
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", input, err)
		}
	}
	if failed != 0 {
		os.Exit(1)
	}
}
