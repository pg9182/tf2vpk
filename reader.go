package tf2vpk

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)

// Reader reads Titanfall 2 VPKs.
type Reader struct {
	Root  ValvePakDir
	block map[ValvePakIndex]io.ReaderAt
	close map[ValvePakIndex]io.Closer
}

// NewReader creates a new Reader reading from vpk.
func NewReader(vpk ValvePakRef) (*Reader, error) {
	return NewReaderFunc(func(i ValvePakIndex) (io.ReaderAt, error) {
		return os.Open(vpk.Resolve(i))
	})
}

// NewReaderFunc creates a new Reader reading using the provided function. If
// the returned [io.ReaderAt] implements [io.Closer], it will be called when the
// Reader is closed.
func NewReaderFunc(open func(ValvePakIndex) (io.ReaderAt, error)) (*Reader, error) {
	r := &Reader{
		block: map[ValvePakIndex]io.ReaderAt{},
		close: map[ValvePakIndex]io.Closer{},
	}

	// read dir index
	dir, err := open(ValvePakIndexDir)
	if err != nil {
		return nil, fmt.Errorf("open vpk dir index: %w", err)
	}
	if err := r.Root.Deserialize(io.NewSectionReader(dir, 0, 1<<63-1)); err != nil {
		return nil, fmt.Errorf("read root directory: %w", err)
	}

	// add dir block
	chunkOffset, err := r.Root.ChunkOffset()
	if err != nil {
		return nil, fmt.Errorf("get chunk offset from root directory: %w", err)
	}
	if dir, ok := dir.(io.Closer); ok {
		r.close[ValvePakIndexDir] = dir
	}
	r.block[ValvePakIndexDir] = io.NewSectionReader(dir, int64(chunkOffset), 1<<63-1)

	// open blocks
	var errs []error
	for _, b := range r.Root.File {
		if _, ok := r.block[b.Index]; !ok {
			if x, err := open(b.Index); err != nil {
				errs = append(errs, fmt.Errorf("open vpk block %s: %w", b.Index, err))
			} else {
				if x, ok := x.(io.Closer); ok {
					r.close[b.Index] = x
				}
				r.block[b.Index] = x
			}
		}
	}
	if err := errors.Join(errs...); err != nil {
		_ = r.Close()
		return nil, fmt.Errorf("open blocks: %w", err)
	}
	return r, nil
}

// Close cleans files opened by the Reader.
func (r *Reader) Close() error {
	var errs []error
	if r.close != nil {
		for i, x := range r.close {
			if err := x.Close(); err != nil {
				errs = append(errs, fmt.Errorf("close data reader for index %d: %w", i, err))
			}
		}
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("close data readers: %w", err)
	}
	return nil
}

// OpenFile returns a new reader reading the contents of a specific file. The checksum is verified at EOF.
func (r *Reader) OpenFile(f ValvePakFile) (io.Reader, error) {
	return f.CreateReader(r.block[f.Index])
}

// OpenFileParallel is like OpenFile, but but decompresses chunks in parallel
// using n goroutines going no more than n compressed chunks ahead.
func (r *Reader) OpenFileParallel(f ValvePakFile, n int) (io.Reader, error) {
	return f.CreateReaderParallel(r.block[f.Index], n)
}

// OpenChunk returns a new reader reading the contents of a specific chunk.
func (r *Reader) OpenChunk(f ValvePakFile, c ValvePakChunk) (io.Reader, error) {
	return c.CreateReader(r.block[f.Index])
}

// OpenChunkRaw returns a new reader reading the raw contents of a specific chunk.
func (r *Reader) OpenChunkRaw(f ValvePakFile, c ValvePakChunk) (io.Reader, error) {
	return c.CreateReaderRaw(r.block[f.Index])
}

// OpenBlockRaw opens a new reader reading the contents of a specific block.
func (r *Reader) OpenBlockRaw(n ValvePakIndex) (io.ReaderAt, error) {
	x, ok := r.block[n]
	if !ok {
		return nil, fmt.Errorf("block %#v out of range", n)
	}
	return x, nil
}

var (
	_ fs.FS          = (*Reader)(nil)
	_ fs.File        = (*readerFile)(nil)
	_ fs.ReadDirFile = (*readerDir)(nil)
	_ fs.DirEntry    = (*readerInfo)(nil)
	_ fs.FileInfo    = (*readerInfo)(nil)
)

type readerFile struct {
	info readerInfo
	rc   io.ReadCloser
}

func (f *readerFile) Stat() (fs.FileInfo, error) {
	return &f.info, nil
}

func (f *readerFile) Read(b []byte) (n int, err error) {
	return f.rc.Read(b)
}

func (f *readerFile) Close() error {
	return f.rc.Close()
}

type readerDir struct {
	info   readerInfo
	entry  []*readerInfo
	offset int
}

func (f *readerDir) Stat() (fs.FileInfo, error) {
	return &f.info, nil
}

func (f *readerDir) Read(b []byte) (n int, err error) {
	return 0, &fs.PathError{Op: "read", Path: f.info.name, Err: fs.ErrInvalid}
}

func (f *readerDir) Close() error {
	return nil
}

func (d *readerDir) ReadDir(count int) ([]fs.DirEntry, error) {
	n := len(d.entry) - d.offset
	if n == 0 && count > 0 {
		return nil, io.EOF
	}
	if count > 0 && n > count {
		n = count
	}
	list := make([]fs.DirEntry, n)
	for i := range list {
		list[i] = d.entry[d.offset+i]
	}
	d.offset += n
	return list, nil
}

type readerInfo struct {
	name string
	file *ValvePakFile
}

func (i *readerInfo) Info() (fs.FileInfo, error) {
	return i, nil
}

func (i *readerInfo) Type() fs.FileMode {
	return i.Mode().Type()
}

func (i *readerInfo) Name() string {
	return i.name
}

func (i *readerInfo) Size() int64 {
	var sz uint64
	if !i.IsDir() {
		for _, c := range i.file.Chunk {
			sz += c.UncompressedSize
		}
	}
	return int64(sz)
}

func (i *readerInfo) Mode() fs.FileMode {
	if i.IsDir() {
		return 0777 | fs.ModeDir
	}
	return 0666
}

func (i *readerInfo) ModTime() time.Time {
	return time.Time{}
}

func (i *readerInfo) IsDir() bool {
	return i.file == nil
}

func (i *readerInfo) Sys() interface{} {
	if i.IsDir() {
		return nil
	}
	return *i.file
}

// Open implements fs.FS.
func (r *Reader) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	name = strings.TrimPrefix(name, "./")
	for fi, f := range r.Root.File {
		if f.Path == name {
			if rc, err := r.OpenFile(f); err != nil {
				return nil, &fs.PathError{Op: "open", Path: name, Err: err}
			} else {
				return &readerFile{readerInfo{path.Base(name), &r.Root.File[fi]}, io.NopCloser(rc)}, nil
			}
		}
	}
	things := map[string]*ValvePakFile{}
	if name == "." {
		for fi, f := range r.Root.File {
			if i := strings.Index(f.Path, "/"); i < 0 {
				things[f.Path] = &r.Root.File[fi]
			} else {
				things[f.Path[:i]] = nil
			}
		}
	} else {
		prefix := name + "/"
		for fi, f := range r.Root.File {
			if strings.HasPrefix(f.Path, prefix) {
				tmp := f.Path[len(prefix):]
				if i := strings.Index(tmp, "/"); i < 0 {
					things[tmp] = &r.Root.File[fi]
				} else {
					things[tmp[:i]] = nil
				}
			}
		}
		if len(things) == 0 {
			return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist} // no file with the provided name, and the name isn't a dir prefix of other files
		}
	}
	var dirents []*readerInfo
	for thing, file := range things {
		dirents = append(dirents, &readerInfo{thing, file})
	}
	sort.Slice(dirents, func(i, j int) bool {
		return dirents[i].name < dirents[j].name
	})
	return &readerDir{readerInfo{name[strings.LastIndex(name, "/")+1:], nil}, dirents, 0}, nil
}
