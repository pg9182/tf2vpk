package tf2vpk

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Reader reads Titanfall 2 VPKs.
type Reader struct {
	Root ValvePakDir
	data map[uint16]interface {
		io.ReaderAt
		io.Closer
	}
}

// OpenReader opens the Titanfall 2 VPK in path with the provided name and root directory prefix.
func OpenReader(path, prefix, name string) (*Reader, error) {
	f, err := os.Open(filepath.Join(path, ValvePakDirName(prefix, name)))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return NewReader(f, func(i uint16) (interface {
		io.ReaderAt
		io.Closer
	}, error) {
		return os.Open(filepath.Join(path, ValvePakBlockName(name, i)))
	})
}

// OpenReaderPath is like OpenReader, but takes the full path to a VPK.
func OpenReaderPath(path, prefix string) (*Reader, error) {
	path, filename := filepath.Split(filepath.FromSlash(path))

	name, ok := strings.CutSuffix(filename, ".vpk")
	if !ok {
		return nil, fmt.Errorf("vpk %q name does not have .vpk extension", filename)
	}

	if len(name) < 4 || name[len(name)-4] != '_' {
		return nil, fmt.Errorf("vpk %q name does not end with _XXX.vpk where XXX is the index or 'dir'", filename)
	}
	name, suffix := name[:len(name)-4], name[len(name)-4:]

	if suffix == "_dir" {
		name, ok = strings.CutPrefix(name, prefix)
		if !ok {
			return nil, fmt.Errorf("vpk %q name does not begin with prefix %q", filename, prefix)
		}
	}

	return OpenReader(path, prefix, name)
}

// NewReader creates a new Reader reading the root directory from dir, and opening the packed chunks from the provided
// function.
func NewReader(dir io.Reader, data func(uint16) (interface {
	io.ReaderAt
	io.Closer
}, error)) (*Reader, error) {
	r := &Reader{
		data: map[uint16]interface {
			io.ReaderAt
			io.Closer
		}{},
	}
	if err := r.Root.Deserialize(dir); err != nil {
		return nil, fmt.Errorf("read root directory: %w", err)
	}
	for _, b := range r.Root.File {
		if _, ok := r.data[b.Index]; !ok {
			if x, err := data(b.Index); err != nil {
				return nil, fmt.Errorf("open raw data reader for index %d: %w", b.Index, err)
			} else {
				r.data[b.Index] = x
			}
		}
	}
	return r, nil
}

// Close cleans files opened by the Reader.
func (r *Reader) Close() error {
	var errs []error
	if r.data != nil {
		for i, x := range r.data {
			if err := x.Close(); err != nil {
				errs = append(errs, fmt.Errorf("close data reader for index %d: %w", i, err))
			}
		}
	}
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		return fmt.Errorf("close data readers: %q", errs)
	}
}

// OpenFile returns a new reader reading the contents of a specific file. The checksum is verified at EOF.
func (r *Reader) OpenFile(f ValvePakFile) (io.Reader, error) {
	return f.CreateReader(r.data[f.Index])
}

// OpenFileParallel is like OpenFile, but but decompresses chunks in parallel
// using n goroutines going no more than n compressed chunks ahead.
func (r *Reader) OpenFileParallel(f ValvePakFile, n int) (io.Reader, error) {
	return f.CreateReaderParallel(r.data[f.Index], n)
}

// OpenChunkRaw returns a new reader reading the contents of a specific chunk.
func (r *Reader) OpenChunkRaw(f ValvePakFile, c ValvePakChunk) (io.Reader, error) {
	return c.CreateReaderRaw(r.data[f.Index])
}

// OpenBlockRaw opens a new reader reading the contents of a specific block.
func (r *Reader) OpenBlockRaw(n uint16) (io.ReaderAt, error) {
	x, ok := r.data[n]
	if !ok {
		return nil, fmt.Errorf("block %d out of range", n)
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
	if strings.HasPrefix(name, "./") {
		name = name[2:]
	}
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
