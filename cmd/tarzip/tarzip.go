package tarzip

import (
	"archive/tar"
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"

	"github.com/pg9182/tf2vpk"
	"github.com/pg9182/tf2vpk/cmd/root"
	"github.com/spf13/cobra"
)

var TarCommand = command("tar")
var ZipCommand = command("zip")

func command(format string) *cobra.Command {
	var main func()
	var Flags struct {
		VPK            tf2vpk.ValvePakRef
		IncludeExclude func(tf2vpk.ValvePakFile) (bool, error)
		Output         string
		Chunks         bool
		RawChunks      bool
		Verbose        bool
	}
	var Command = &cobra.Command{
		GroupID: root.GroupVPKRead.ID,
		Use:     format + " vpk_path",
		Short:   "Streams the contents of VPK as a " + format + " archive",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			main()
		},
	}
	main = func() {
		if Flags.RawChunks && !Flags.Chunks {
			fmt.Fprintf(os.Stderr, "error: --raw-chunks requires --chunks\n")
			os.Exit(2)
		}

		r, err := tf2vpk.NewReader(Flags.VPK)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: open vpk: %v\n", err)
			os.Exit(1)
		}

		var w *os.File
		switch Flags.Output {
		case "":
			fmt.Fprintf(os.Stderr, "error: no output file specified\n")
			os.Exit(1)
		case "-":
			w = os.Stdout
		default:
			w, err = os.OpenFile(Flags.Output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: create output file: %v\n", err)
				os.Exit(1)
			}
		}
		defer w.Close()

		var (
			archive func(name string, size int64, r io.Reader) error
			finish  func() error
		)
		switch format {
		case "tar":
			a := tar.NewWriter(w)
			ds := map[string]struct{}{}
			archive = func(name string, size int64, r io.Reader) error {
				var mkdirs []string
			d:
				for d := path.Dir(name); d != "" && d != "."; d = path.Dir(d) {
					if _, ok := ds[d]; ok {
						continue d
					}
					mkdirs = append(mkdirs, d)
					ds[d] = struct{}{}
				}
				for i := len(mkdirs) - 1; i >= 0; i-- {
					if err := a.WriteHeader(&tar.Header{
						Name: mkdirs[i] + "/",
						Mode: 0777,
					}); err != nil {
						return err
					}
				}
				err := a.WriteHeader(&tar.Header{
					Name: name,
					Size: size,
					Mode: 0666,
				})
				if err == nil {
					_, err = io.Copy(a, r)
				}
				return err
			}
			finish = a.Close
		case "zip":
			a := zip.NewWriter(w)
			archive = func(name string, size int64, r io.Reader) error {
				w, err := a.CreateHeader(&zip.FileHeader{
					Name:               name,
					UncompressedSize64: uint64(size),
				})
				if err == nil {
					_, err = io.Copy(w, r)
				}
				return err
			}
			finish = a.Close
		default:
			panic("wtf")
		}
		for _, f := range r.Root.File {
			if skip, err := Flags.IncludeExclude(f); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			} else if skip {
				if Flags.Verbose {
					fmt.Fprintf(os.Stderr, "%s (skipped)\n", f.Path)
				}
				continue
			}
			if Flags.Verbose {
				fmt.Fprintf(os.Stderr, "%s\n", f.Path)
			}
			if Flags.Chunks {
				for i, c := range f.Chunk {
					var (
						ext string
						sz  uint64
						cr  io.Reader
						err error
					)
					if Flags.RawChunks {
						if c.IsCompressed() {
							ext = ".lzham"
						}
						sz = c.CompressedSize
						cr, err = r.OpenChunkRaw(f, c)
					} else {
						sz = c.UncompressedSize
						cr, err = r.OpenChunk(f, c)
					}
					if err != nil {
						fmt.Fprintf(os.Stderr, "error: read vpk file %q: chunk %d: %v\n", f.Path, i, err)
						os.Exit(1)
					}
					if err = archive(f.Path+"/"+strconv.Itoa(i)+ext, int64(sz), cr); err != nil {
						fmt.Fprintf(os.Stderr, "error: process vpk file %q: chunk %d: %v\n", f.Path, i, err)
						os.Exit(1)
					}
				}
			} else {
				var sz uint64
				for _, c := range f.Chunk {
					sz += c.UncompressedSize
				}
				if fr, err := r.OpenFileParallel(f, root.Flags.Threads); err != nil {
					fmt.Fprintf(os.Stderr, "error: read vpk file %q: %v\n", f.Path, err)
					os.Exit(1)
				} else if err = archive(f.Path, int64(sz), fr); err != nil {
					fmt.Fprintf(os.Stderr, "error: process vpk file %q: %v\n", f.Path, err)
					os.Exit(1)
				}
			}
		}
		if err := finish(); err != nil {
			fmt.Fprintf(os.Stderr, "error: write output file %q: %v\n", Flags.Output, err)
			os.Exit(1)
		}

		if err := w.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "error: write output file %q: %v\n", Flags.Output, err)
			os.Exit(1)
		}
	}
	{
		root.ArgVPK(&Flags.VPK, Command, -1, false, false, false)
		root.FlagIncludeExclude(&Flags.IncludeExclude, Command, true)
		Command.Flags().StringVarP(&Flags.Output, "output", "o", "-", "write the archive to a file")
		Command.Flags().BoolVarP(&Flags.Chunks, "chunks", "c", false, "instead of assembling files, make each file a dir, and output the raw chunks as numbered files within")
		Command.Flags().BoolVarP(&Flags.RawChunks, "raw-chunks", "C", false, "do not decompress compressed chunks (requires --chunks)")
		Command.Flags().BoolVarP(&Flags.Verbose, "verbose", "v", false, "display files as they are archived")
		root.Command.AddCommand(Command)
	}
	return Command
}
