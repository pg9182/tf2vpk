// Command tf2-vpkoptim repacks and filters Titanfall 2 VPKs.
package main

import (
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/pg9182/tf2vpk"
	"github.com/pg9182/tf2vpk/internal"
	"github.com/pg9182/tf2vpk/vpkutil"
	"github.com/spf13/pflag"
)

var (
	Output    = pflag.StringP("output", "o", ".", "The output directory (must be different from the input dir)")
	VPKPrefix = pflag.StringP("vpk-prefix", "p", "english", "VPK prefix")
	Verbose   = pflag.CountP("verbose", "v", "Show verbose output (repeat for more verbosity) (1=status, 2=verbose, 3=debug, 4=extra)")
	DryRun    = pflag.BoolP("dry-run", "n", false, "Don't write output files")

	Merge          = pflag.Bool("merge", false, "Merges all blocks (i.e., _XXX.vpk)")
	IncludeExclude = vpkutil.NewCLIIncludeExclude(pflag.CommandLine)

	Help = pflag.BoolP("help", "h", false, "Show this help message")
)

const (
	VStatus  = 1
	VVerbose = 2
	VDebug   = 3
	VDebug1  = 4
)

func vlog(n int, format string, a ...interface{}) {
	if *Verbose >= n {
		fmt.Fprintf(os.Stderr, format+"\n", a...)
	}
}

func main() {
	pflag.Parse()

	argv := pflag.Args()
	if len(argv) < 1 || *Help {
		fmt.Fprintf(os.Stderr, "usage: %s [options] vpk_dir [vpk_name...]\n\noptions:\n%s", os.Args[0], pflag.CommandLine.FlagUsages())
		if !*Help {
			os.Exit(2)
		}
		return
	}

	var inputDir string
	if x, err := filepath.EvalSymlinks(argv[0]); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to resolve input path: %v\n", err)
		os.Exit(1)
	} else if x, err := filepath.Abs(x); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to resolve input path: %v\n", err)
		os.Exit(1)
	} else {
		inputDir = x
	}

	if err := os.Mkdir(*Output, 0777); err != nil && !errors.Is(err, fs.ErrExist) {
		fmt.Fprintf(os.Stderr, "error: failed to create output directory: %v\n", err)
		os.Exit(1)
	}

	var outputDir string
	if x, err := filepath.EvalSymlinks(*Output); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to resolve output path: %v\n", err)
		os.Exit(1)
	} else if x, err := filepath.Abs(x); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to resolve output path: %v\n", err)
		os.Exit(1)
	} else {
		outputDir = x
	}

	if inputDir == outputDir {
		fmt.Fprintf(os.Stderr, "error: output path must be different from the input put\n")
		os.Exit(1)
	}

	vlog(VStatus, "finding vpks...")

	var vpkName []string
	if len(argv) == 1 {
		es, err := os.ReadDir(inputDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: find vpks: %v\n", err)
		}
		for _, e := range es {
			name, idx, err := tf2vpk.SplitName(e.Name(), *VPKPrefix)
			if err != nil {
				continue
			}
			if idx != tf2vpk.ValvePakIndexDir {
				continue
			}
			vlog(VVerbose, "found %s", filepath.Join(inputDir, e.Name()))
			vpkName = append(vpkName, name)
		}
	} else {
		vpkName = argv[1:]
	}

	if *DryRun {
		vlog(VStatus, "dry-run enabled, will not actually write output files")
	}

	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt)
	defer done()

	for _, x := range vpkName {
		vlog(VStatus, "")
		if err := optimize(ctx, inputDir, outputDir, x); err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to optimize %q: %v\n", x, err)
			os.Exit(1)
		}
	}

	vlog(VStatus, "")
	vlog(VStatus, "done")
}

func optimize(ctx context.Context, inputDir, outputDir, vpkName string) error {
	vlog(VStatus, "optimizing %s", filepath.Base(vpkName))

	vpk := tf2vpk.ValvePakRef{Path: inputDir, Prefix: *VPKPrefix, Name: vpkName}

	r, err := tf2vpk.NewReader(vpk)
	if err != nil {
		return err
	}
	defer r.Close()

	var origBlockBytesTotal uint64
	origBlockBytes := map[tf2vpk.ValvePakIndex]uint64{}
	for _, f := range r.Root.File {
		for _, c := range f.Chunk {
			if x := c.Offset + c.CompressedSize; x > origBlockBytes[f.Index] {
				origBlockBytes[f.Index] = x
			}
		}
	}
	for _, x := range origBlockBytes {
		origBlockBytesTotal += x
	}

	// TODO: use interval/segment trees to make this more efficient (time, space, and the resulting size)

	type CID struct {
		BlockIndex  tf2vpk.ValvePakIndex
		ChunkOffset uint64
		ChunkSize   uint64
	}

	chunkHash := map[CID][20]byte{}
	hashChunk := map[[20]byte]CID{}

	if err := vpkHashChunks(ctx, r, func(ctx context.Context, f tf2vpk.ValvePakFile, c tf2vpk.ValvePakChunk, h [20]byte) error {
		cid := CID{f.Index, c.Offset, c.CompressedSize}
		chunkHash[cid] = h
		hashChunk[h] = cid
		return nil
	}); err != nil {
		return fmt.Errorf("hash chunks: %w", err)
	}

	var nf []tf2vpk.ValvePakFile
	var nfd int
	for _, f := range r.Root.File {
		if skip, err := IncludeExclude.Skip(f); err != nil {
			return err
		} else if skip {
			vlog(VVerbose, "--- excluding %s", f.Path)
			nfd++
			continue
		}
		nf = append(nf, f)
	}
	vlog(VStatus, "--- excluding %d files", nfd)
	r.Root.File = nf

	bf := map[tf2vpk.ValvePakIndex]io.Writer{}
	if *Merge {
		tf, err := os.CreateTemp(outputDir, ".vpkblock0")
		if err != nil {
			return fmt.Errorf("create temp file for vpk block 0: %w", err)
		}
		defer os.Remove(tf.Name())

		vlog(VVerbose, "--- created output file for block 0")
		bf[0] = tf
	} else {
		for _, f := range r.Root.File {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if _, ok := bf[f.Index]; ok {
				continue
			}

			if *DryRun {
				bf[f.Index] = io.Discard
			} else {
				tf, err := os.CreateTemp(outputDir, ".vpkblock"+f.Index.String()+"-*")
				if err != nil {
					return fmt.Errorf("create temp file for vpk block %d: %w", f.Index, err)
				}
				defer os.Remove(tf.Name())

				vlog(VVerbose, "--- created output file %s for block %d", tf.Name())
				bf[f.Index] = tf
			}
		}
	}
	if *Merge {
		vlog(VStatus, "--- writing %d block(s) (merged)", len(bf))
	} else {
		vlog(VStatus, "--- writing %d block(s)", len(bf))
	}

	bfc := make(map[tf2vpk.ValvePakIndex]uint64, len(bf))              // current offset
	bfw := make(map[tf2vpk.ValvePakIndex]map[[20]byte]uint64, len(bf)) // written chunk offset

	for x := range bf {
		bfc[x] = 0
		bfw[x] = map[[20]byte]uint64{}
	}

	var cc int
	var ccb int64
	for i, f := range r.Root.File {
		var targetIndex tf2vpk.ValvePakIndex
		if *Merge {
			targetIndex = 0
		} else {
			targetIndex = f.Index
		}
		if targetIndex == tf2vpk.ValvePakIndexDir {
			panic("tf2-vpkoptim: writing chunks after index dir not implemented yet")
		}
		for j, c := range f.Chunk {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			cid := CID{f.Index, c.Offset, c.CompressedSize}
			ch := chunkHash[cid]

			if x, ok := bfw[targetIndex][ch]; ok {
				vlog(VDebug1, "--- found chunk %#v in target block %d with hash %x at offset %d", cid, targetIndex, ch, x)
				r.Root.File[i].Chunk[j].Offset = x
				continue
			}

			cr, err := r.OpenChunkRaw(f, c)
			if err != nil {
				return fmt.Errorf("write chunk %#v to target block %d with hash %d: read original chunk: %w", cid, targetIndex, ch, err)
			}

			n, err := io.Copy(bf[targetIndex], cr)
			if err != nil {
				return fmt.Errorf("write chunk %#v to target block %d with hash %d: copy data: %w", cid, targetIndex, ch, err)
			}

			co := bfc[targetIndex]
			bfw[targetIndex][ch] = co
			r.Root.File[i].Chunk[j].Offset = co

			bfc[targetIndex] += uint64(n)
			ccb += n

			vlog(VDebug1, "--- wrote chunk %#v to target block %d with hash %d to offset %d", cid, targetIndex, ch, co)
			cc++
		}
		r.Root.File[i].Index = targetIndex
	}
	vlog(VStatus, "--- wrote %d chunks (%s; delta %s)", cc, internal.FormatBytesSI(ccb), internal.FormatBytesSI(ccb-int64(origBlockBytesTotal)))

	if *DryRun {
		if err := r.Root.Serialize(io.Discard); err != nil {
			return fmt.Errorf("write vpk dir: %w", err)
		}
		vlog(VStatus, "--- wrote vpk dir (%d files; delta %d)", len(r.Root.File), -1*nfd)
	} else {
		df, err := os.CreateTemp(outputDir, ".vpkdir-*")
		if err != nil {
			return fmt.Errorf("create temp file for vpk dir: %w", err)
		}
		defer os.Remove(df.Name())

		vlog(VVerbose, "--- created output file %s for dir", df.Name())

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := r.Root.Serialize(df); err != nil {
			return fmt.Errorf("write vpk dir: %w", err)
		}
		vlog(VStatus, "--- wrote vpk dir (%d files; delta %d)", len(r.Root.File), -1*nfd)

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := df.Sync(); err != nil {
			return fmt.Errorf("write vpk dir: %w", err)
		}
		if err := df.Close(); err != nil {
			return fmt.Errorf("write vpk dir: %w", err)
		}

		for n, x := range bf {
			if err := x.(*os.File).Sync(); err != nil {
				return fmt.Errorf("write vpk block %d: %w", n, err)
			}
			if err := x.(*os.File).Close(); err != nil {
				return fmt.Errorf("write vpk dir %d: %w", n, err)
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := os.Rename(df.Name(), filepath.Join(outputDir, tf2vpk.JoinName(*VPKPrefix, vpkName, tf2vpk.ValvePakIndexDir))); err != nil {
			return fmt.Errorf("rename final vpk dir: %w", err)
		}
		vlog(VDebug, "saved vpk dir %s", tf2vpk.JoinName(*VPKPrefix, vpkName, tf2vpk.ValvePakIndexDir))

		for n, x := range bf {
			if err := os.Rename(x.(*os.File).Name(), filepath.Join(outputDir, tf2vpk.JoinName(*VPKPrefix, vpkName, n))); err != nil {
				return fmt.Errorf("rename final vpk block: %w", err)
			}
			vlog(VDebug, "saved vpk block %s", tf2vpk.JoinName(*VPKPrefix, vpkName, n))
		}
	}

	return nil
}

func vpkHashChunks(ctx context.Context, r *tf2vpk.Reader, fn func(ctx context.Context, f tf2vpk.ValvePakFile, c tf2vpk.ValvePakChunk, h [20]byte) error) error {
	for _, f := range r.Root.File {
		for _, c := range f.Chunk {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			cr, err := r.OpenChunkRaw(f, c)
			if err != nil {
				panic(err)
			}

			h := sha1.New()
			if _, err := io.Copy(h, cr); err != nil {
				panic(err)
			}

			var s [sha1.Size]byte
			h.Sum(s[:0])

			if fn != nil {
				if err := fn(ctx, f, c, s); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
