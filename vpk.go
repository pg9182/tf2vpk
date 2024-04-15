// Package tf2vpk manipulates Titanfall 2 VPKs.
package tf2vpk

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/pg9182/tf2lzham"
)

// Titanfall 2 VPK constants.
const (
	ValvePakMagic                    uint32 = 0x55AA1234
	ValvePakVersionMajor             uint16 = 2
	ValvePakVersionMinor             uint16 = 3
	ValvePakMaxChunkUncompressedSize uint64 = 0x100000
)

// ValvePakDir is the root directory of a Titanfall 2 VPK, providing
// byte-for-byte identical serialization/deserialization and validation (it will
// refuse to read or write invalid structs).
type ValvePakDir struct {
	Magic        uint32
	MajorVersion uint16
	MinorVersion uint16
	treeSize     uint32 // will be dynamically calculated when writing
	DataSize     uint32
	File         []ValvePakFile
}

// Deserialize parses a ValvePakDir from r.
func (d *ValvePakDir) Deserialize(r io.Reader) error {
	if err := binary.Read(r, binary.LittleEndian, &d.Magic); err != nil {
		return fmt.Errorf("read dir magic: %w", err)
	} else if d.Magic != ValvePakMagic {
		return fmt.Errorf("read magic: expected %08X, got %08X", ValvePakMagic, d.Magic)
	}
	if err := binary.Read(r, binary.LittleEndian, &d.MajorVersion); err != nil {
		return fmt.Errorf("read major version: %w", err)
	} else if err := binary.Read(r, binary.LittleEndian, &d.MinorVersion); err != nil {
		return fmt.Errorf("read minor version: %w", err)
	} else if d.MajorVersion != ValvePakVersionMajor || d.MinorVersion != ValvePakVersionMinor {
		return fmt.Errorf("unsupported dir version %d.%d (expected %d.%d)", d.MajorVersion, d.MinorVersion, ValvePakVersionMajor, ValvePakVersionMinor)
	}
	if err := binary.Read(r, binary.LittleEndian, &d.treeSize); err != nil {
		return fmt.Errorf("read tree size: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &d.DataSize); err != nil {
		return fmt.Errorf("read data size: %w", err)
	} else if d.DataSize != 0 {
		return fmt.Errorf("preload bytes are not implemented (and they shouldn't be in the TF2 VPKs anyways)")
	}
	// note: there isn't really any required order to the tree items as long as the ext/path/name is grouped together (the game builds a lookup table itself when reading the vpk)
	b := bufio.NewReader(io.LimitReader(r, int64(d.treeSize)))
	for {
		xx, err := readNullString(b)
		if err != nil {
			return fmt.Errorf("read directory tree extension: %w", err)
		}
		if xx == "" {
			break
		}
		for {
			xp, err := readNullString(b)
			if err != nil {
				return fmt.Errorf("read directory tree path: %w", err)
			}
			if xp == "" {
				break
			}
			for {
				xn, err := readNullString(b)
				if err != nil {
					return fmt.Errorf("read directory tree name: %w", err)
				}
				if xn == "" {
					break
				}
				var fn string
				if xp == " " {
					fn = xn + "." + xx
				} else {
					fn = xp + "/" + xn + "." + xx
				}
				var f ValvePakFile
				if err := f.Deserialize(b, fn); err != nil {
					return fmt.Errorf("read directory tree file data for %q: %w", f.Path, err)
				}
				//fmt.Println(xx, xp, xn)
				d.File = append(d.File, f)
			}
		}
	}
	if _, err := b.Peek(1); err != io.EOF {
		return fmt.Errorf("read directory tree: expected tree size %d, but tree ended before that", d.treeSize)
	}
	if x, err := d.TreeSize(); err != nil {
		panic(fmt.Errorf("serialized tree size mismatch: failed to serialize: %w (this is a bug in the serialization or a mismatch in the validation logic)", err))
	} else if x != d.treeSize {
		panic(fmt.Errorf("serialized tree size mismatch: expected %d, got %d (this is a bug in the serialization)", d.treeSize, x))
	}
	return nil
}

func readNullString(r io.ByteReader) (string, error) {
	var s []byte
	for {
		b, err := r.ReadByte()
		if err != nil {
			return string(s), err
		}
		if b == 0 {
			break
		}
		s = append(s, b)
	}
	return string(s), nil
}

// Serialize writes an encoded ValvePakDir to w. The output should be identical
// byte-for-byte.
func (d ValvePakDir) Serialize(w io.Writer) error {
	ts, err := d.TreeSize()
	if err != nil {
		return fmt.Errorf("calculate tree size: %w", err)
	}
	if d.Magic != ValvePakMagic {
		return fmt.Errorf("write magic: expected %08X, got %08X", ValvePakMagic, d.Magic)
	} else if err := binary.Write(w, binary.LittleEndian, &d.Magic); err != nil {
		return fmt.Errorf("write dir magic: %w", err)
	}
	if d.MajorVersion != ValvePakVersionMajor || d.MinorVersion != ValvePakVersionMinor {
		return fmt.Errorf("unsupported dir version %d.%d (expected %d.%d)", d.MajorVersion, d.MinorVersion, ValvePakVersionMajor, ValvePakVersionMinor)
	} else if err := binary.Write(w, binary.LittleEndian, &d.MajorVersion); err != nil {
		return fmt.Errorf("write major version: %w", err)
	} else if err := binary.Write(w, binary.LittleEndian, &d.MinorVersion); err != nil {
		return fmt.Errorf("write minor version: %w", err)
	}
	if err := binary.Write(w, binary.LittleEndian, ts); err != nil {
		return fmt.Errorf("write tree size: %w", err)
	}
	if d.DataSize != 0 {
		return fmt.Errorf("preload bytes are not implemented (and they shouldn't be in the TF2 VPKs anyways)")
	} else if err := binary.Write(w, binary.LittleEndian, &d.DataSize); err != nil {
		return fmt.Errorf("write data size: %w", err)
	}
	if err := d.writeTree(w); err != nil {
		return fmt.Errorf("write directory tree: %w", err)
	}
	return nil
}

// SortFiles sorts the files in an order suitable for the tree.
func (d *ValvePakDir) SortFiles() error {
	sp := make(map[string][3]string, len(d.File))
	for _, f := range d.File {
		ext, path, base, err := splitPath(f.Path)
		if err != nil {
			return fmt.Errorf("write directory tree: sort files: %w", err)
		}
		sp[f.Path] = [3]string{ext, path, base}
	}
	sort.Slice(d.File, func(i, j int) bool {
		for k := range sp[d.File[i].Path] {
			if a, b := sp[d.File[i].Path][k], sp[d.File[j].Path][k]; a != b {
				return a < b
			}
		}
		return false
	})
	return nil
}

func splitPath(p string) (ext, path, base string, err error) {
	i1 := strings.LastIndex(p, "/")
	if i1 == -1 {
		p = " /" + p
		i1 = 1
	}
	i2 := strings.LastIndex(p[i1:], ".")
	if i2 == -1 {
		return "", "", "", fmt.Errorf("no extension for file %q", p)
	}
	return p[i1+i2+1:], p[:i1], p[i1+1 : i1+i2], nil
}

func (d ValvePakDir) TreeSize() (uint32, error) {
	var b countWriter
	if err := d.writeTree(&b); err != nil {
		return 0, err
	}
	return uint32(b.N), nil
}

// ChunkOffset returns the starting offset of chunk data stored after the dir
// index (i.e., add this to the ValvePakChunk.Offset when reading a chunk for a
// file with ValvePakFile.Index == ValvePakIndexDir).
func (d ValvePakDir) ChunkOffset() (n uint32, err error) {
	n += uint32(binary.Size(d.Magic))
	n += uint32(binary.Size(d.MajorVersion))
	n += uint32(binary.Size(d.MinorVersion))
	n += uint32(binary.Size(d.treeSize))
	n += uint32(binary.Size(d.DataSize))

	treeSize, err := d.TreeSize()
	if err == nil {
		n += treeSize
	}
	return n, err
}

type countWriter struct {
	N int64
}

func (c *countWriter) Write(b []byte) (n int, err error) {
	n = len(b)
	c.N += int64(n)
	return
}

func (d ValvePakDir) writeTree(w io.Writer) error {
	var seenExt, seenPath, seenBase map[string]struct{}
	lastExt, lastPath, lastBase := "\xFF", "\xFF", "\xFF"

	seenExt = map[string]struct{}{}
	for _, f := range d.File {
		ext, path, base, err := splitPath(f.Path)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		if lastExt != ext {
			if _, seen := seenExt[ext]; !seen {
				if lastPath != "\xFF" {
					if _, err := w.Write([]byte{'\x00'}); err != nil {
						return fmt.Errorf("end path branch %s/%s: %w", lastExt, lastPath, err)
					}
				}
				if lastExt != "\xFF" {
					if _, err := w.Write([]byte{'\x00'}); err != nil {
						return fmt.Errorf("end ext branch %s: %w", lastExt, err)
					}
				}
				seenExt[ext] = struct{}{}
				seenPath, lastPath = map[string]struct{}{}, "\xFF"
				seenBase, lastBase = map[string]struct{}{}, "\xFF"
			} else {
				return fmt.Errorf("start new ext branch %s: not sorted correctly: already seen", ext)
			}
			if _, err := w.Write(append([]byte(ext), '\x00')); err != nil {
				return fmt.Errorf("start new ext branch %s: %w", ext, err)
			}
		}
		if lastPath != path {
			if _, seen := seenPath[path]; !seen {
				if lastPath != "\xFF" {
					if _, err := w.Write([]byte{'\x00'}); err != nil {
						return fmt.Errorf("end path branch %s/%s: %w", lastExt, lastPath, err)
					}
				}
				seenPath[path] = struct{}{}
				seenBase, lastBase = map[string]struct{}{}, "\xFF"
			} else {
				return fmt.Errorf("start new path branch %s/%s: not sorted correctly: already seen", ext, path)
			}
			if _, err := w.Write(append([]byte(path), '\x00')); err != nil {
				return fmt.Errorf("start new path branch %s/%s: %w", ext, path, err)
			}
		}
		if lastBase != base {
			if _, seen := seenBase[base]; !seen {
				seenBase[base] = struct{}{}
			} else {
				return fmt.Errorf("add file node %s/%s/%s: not sorted correctly: already seen", ext, path, base)
			}
			if _, err := w.Write(append([]byte(base), '\x00')); err != nil {
				return fmt.Errorf("add file node %s/%s/%s: %w", ext, path, base, err)
			}
			if err := f.Serialize(w); err != nil {
				return fmt.Errorf("add file node %s/%s/%s: %w", ext, path, base, err)
			}
		}
		lastExt, lastPath, lastBase = ext, path, base
	}
	if lastPath != "\xFF" {
		if _, err := w.Write([]byte{'\x00'}); err != nil {
			return fmt.Errorf("end path branch %q/%q: %w", lastExt, lastPath, err)
		}
	}
	if lastExt != "\xFF" {
		if _, err := w.Write([]byte{'\x00'}); err != nil {
			return fmt.Errorf("end ext branch %q: %w", lastExt, err)
		}
	}
	if _, err := w.Write([]byte{'\x00'}); err != nil {
		return fmt.Errorf("end tree: %w", err)
	}
	return nil
}

// ValvePakIndex is an VPK block index.
type ValvePakIndex uint16

const (
	ValvePakIndexDir ValvePakIndex = 0x7FFF // refers to data after the index in _dir.vpk
	ValvePakIndexEOF ValvePakIndex = 0xFFFF // not actually one
)

func (i ValvePakIndex) String() string {
	switch i {
	case ValvePakIndexDir:
		return "dir"
	case ValvePakIndexEOF:
		return "EOF"
	default:
		return fmt.Sprintf("%03d", i)
	}
}

func (i ValvePakIndex) GoString() string {
	switch i {
	case ValvePakIndexDir:
		return "ValvePakIndexDir"
	case ValvePakIndexEOF:
		return "ValvePakIndexEOF"
	default:
		return "ValvePakIndex(" + strconv.FormatUint(uint64(i), 10) + ")"
	}
}

// ValvePakFile is a file in a Titanfall 2 VPK.
type ValvePakFile struct {
	Path         string
	CRC32        uint32
	PreloadBytes uint16
	Index        ValvePakIndex
	Chunk        []ValvePakChunk
}

// LoadFlags gets the load flags for the file.
func (f *ValvePakFile) LoadFlags() (uint32, error) {
	if len(f.Chunk) == 0 {
		return 0, fmt.Errorf("invalid file: no chunks")
	}
	for _, c := range f.Chunk {
		if c.LoadFlags != f.Chunk[0].LoadFlags {
			return 0, fmt.Errorf("invalid file: load flags don't match for all chunks")
		}
	}
	return f.Chunk[0].LoadFlags, nil
}

// TextureFlags gets the texture flags for the file.
func (f *ValvePakFile) TextureFlags() (uint16, error) {
	if len(f.Chunk) == 0 {
		return 0, fmt.Errorf("invalid file: no chunks")
	}
	for _, c := range f.Chunk {
		if uint32(c.TextureFlags) != uint32(f.Chunk[0].TextureFlags) {
			return 0, fmt.Errorf("invalid file: texture flags don't match for all chunks")
		}
	}
	return f.Chunk[0].TextureFlags, nil
}

// CreateReader creates a new reader for the file, checking the CRC32 at EOF.
func (f *ValvePakFile) CreateReader(r io.ReaderAt) (io.Reader, error) {
	return f.CreateReaderParallel(r, 1)
}

// CreateReaderParallel is like CreateReader, but decompresses chunks in
// parallel using n-1 goroutines going no more than n compressed chunks ahead
// (i.e., 1 is not parallel).
func (f *ValvePakFile) CreateReaderParallel(r io.ReaderAt, n int) (io.Reader, error) {
	rs := make([]io.Reader, len(f.Chunk))
	var sz uint64
	var err error
	for i, c := range f.Chunk {
		rs[i], err = c.CreateReader(r)
		if err != nil {
			return nil, fmt.Errorf("chunk %d: %w", i, err)
		}
		sz += c.UncompressedSize
	}
	return newCRCReader(newMultiChunkReader(n-1, rs...), sz, f.CRC32), nil
}

type multiChunkReader struct {
	readers  []io.Reader
	parallel int
}

func newMultiChunkReader(parallel int, rd ...io.Reader) *multiChunkReader {
	return &multiChunkReader{readers: rd, parallel: parallel}
}

func (mr *multiChunkReader) Read(p []byte) (n int, err error) {
	for len(mr.readers) > 0 {
		n, err = mr.readers[0].Read(p)
		if err == io.EOF {
			mr.readers[0], mr.readers = nil, mr.readers[1:]

			if ahead := mr.parallel; ahead > 0 {
				for _, r := range mr.readers {
					if r, ok := r.(interface {
						// EnsureDecompressed synchronously decompresses the
						// block if needed. It must be safe to be called in
						// concurrently with Read.
						EnsureDecompressed() error
					}); ok {
						go r.EnsureDecompressed()
						if ahead--; ahead == 0 {
							break
						}
					}
				}
			}
		}
		if n > 0 || err != io.EOF {
			if err == io.EOF && len(mr.readers) > 0 {
				err = nil
			}
			return
		}
	}
	return 0, io.EOF
}

func (mr *multiChunkReader) WriteTo(w io.Writer) (sum int64, err error) {
	buf := make([]byte, 1024*32)
	for i, r := range mr.readers {
		var n int64
		n, err = io.CopyBuffer(w, r, buf)
		sum += n
		if err != nil {
			mr.readers = mr.readers[i:]
			return sum, err
		}
		mr.readers[i] = nil
	}
	mr.readers = nil
	return sum, nil
}

// Deserialize parses a ValvePakFile from r.
func (f *ValvePakFile) Deserialize(r io.Reader, path string) error {
	f.Path = path
	if err := binary.Read(r, binary.LittleEndian, &f.CRC32); err != nil {
		return fmt.Errorf("read file crc32: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &f.PreloadBytes); err != nil {
		return fmt.Errorf("read file preload bytes: %w", err)
	} else if f.PreloadBytes != 0 {
		return fmt.Errorf("non-zero preload bytes are not implemented (and they shouldn't be in the TF2 VPKs anyways)")
	}
	if err := binary.Read(r, binary.LittleEndian, &f.Index); err != nil {
		return fmt.Errorf("read file archive index: %w", err)
	}
	for {
		var e ValvePakChunk
		if err := e.Deserialize(r); err != nil {
			return fmt.Errorf("read file chunk: %w", err)
		}
		f.Chunk = append(f.Chunk, e)

		// assumptions based on observation
		if e.LoadFlags != f.Chunk[0].LoadFlags {
			return fmt.Errorf("read file chunk: expected load flags to be the same for all chunks")
		}
		if e.TextureFlags != f.Chunk[0].TextureFlags {
			return fmt.Errorf("read file chunk: expected texture flags to be the same for all chunks")
		}
		if e.UncompressedSize > ValvePakMaxChunkUncompressedSize {
			return fmt.Errorf("read file chunk: uncompressed size %d larger than %d", e.UncompressedSize, ValvePakMaxChunkUncompressedSize) // I'm not 100% sure about this limit
		}

		var n ValvePakIndex
		if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
			return fmt.Errorf("read file chunk terminator: %w", err)
		}
		if n == ValvePakIndexEOF {
			break
		} else if n != f.Index {
			return fmt.Errorf("non-eof chunk terminator must equal the block index") // assumption based on observation
		}
	}
	return nil
}

// Serialize writes an encoded ValvePakFile to w.
func (f ValvePakFile) Serialize(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, &f.CRC32); err != nil {
		return fmt.Errorf("write file crc32: %w", err)
	}
	if f.PreloadBytes != 0 {
		return fmt.Errorf("non-zero preload bytes are not implemented (and they shouldn't be in the TF2 VPKs anyways)")
	} else if err := binary.Write(w, binary.LittleEndian, &f.PreloadBytes); err != nil {
		return fmt.Errorf("write file preload bytes: %w", err)
	}
	if err := binary.Write(w, binary.LittleEndian, &f.Index); err != nil {
		return fmt.Errorf("write file archive index: %w", err)
	}
	for i, e := range f.Chunk {
		// assumptions based on observation
		if f.Path != "" && e.TextureFlags != 0 && !strings.HasSuffix(f.Path, ".vtf") {
			return fmt.Errorf("write file chunk: expected non-vtf to not have texture flags")
		}
		if e.LoadFlags != f.Chunk[0].LoadFlags {
			return fmt.Errorf("write file chunk: expected load flags to be the same for all chunks")
		}
		if e.TextureFlags != f.Chunk[0].TextureFlags {
			return fmt.Errorf("write file chunk: expected texture flags to be the same for all chunks")
		}
		if e.UncompressedSize > ValvePakMaxChunkUncompressedSize {
			return fmt.Errorf("write file chunk: uncompressed size %d larger than %d", e.UncompressedSize, ValvePakMaxChunkUncompressedSize) // I'm not 100% sure about this limit
		}

		if i != 0 {
			if err := binary.Write(w, binary.LittleEndian, f.Index); err != nil {
				return fmt.Errorf("write file chunk terminator: %w", err)
			}
		}
		if err := e.Serialize(w); err != nil {
			return fmt.Errorf("write file chunk: %w", err)
		}
	}
	if err := binary.Write(w, binary.LittleEndian, uint16(65535)); err != nil {
		return fmt.Errorf("write file eof chunk terminator: %w", err)
	}
	return nil
}

// Names for flag 1<<index. Based on https://github.com/barnabwhy/sourcepak-rs/blob/fb475240380851463bde3140f01b968d8b2e02c0/src/pak/revpk/format.rs#L93-L109.
var (
	loadFlags = [32]string{
		0:  "VISIBLE",
		8:  "CACHE",
		10: "ACACHE_UNK0",
		18: "TEXTURE_UNK0",
		19: "TEXTURE_UNK1",
		20: "TEXTURE_UNK2",
	}
	textureFlags = [16]string{
		3:  "DEFAULT",
		10: "ENVIRONMENT_MAP",
	}
)

// DescribeLoadFlags returns a human-readable slice of strings describing the
// provided load flags.
func DescribeLoadFlags(flags uint32) (s []string) {
	for i, x := range loadFlags {
		if flags&(uint32(1)<<i) != 0 {
			if x != "" {
				x = ":" + x
			}
			s = append(s, fmt.Sprintf("%02d%s", i, x))
		}
	}
	return
}

// DescribeTextureFlags returns a human-readable slice of strings describing the
// provided texture flags.
func DescribeTextureFlags(flags uint16) (s []string) {
	for i, x := range textureFlags {
		if flags&(uint16(1)<<i) != 0 {
			if x != "" {
				x = ":" + x
			}
			s = append(s, fmt.Sprintf("%02d%s", i, x))
		}
	}
	return
}

// ValvePakChunk is a file chunk (possibly shared) in a Titanfall 2 VPK.
type ValvePakChunk struct {
	LoadFlags        uint32 // note: these flags seem to be the same for all chunks in a ValvePakFile
	TextureFlags     uint16 // ^, and these ones only seem to be on VTF files
	Offset           uint64
	CompressedSize   uint64
	UncompressedSize uint64
}

// IsCompressed checks if a chunk is compressed.
func (c ValvePakChunk) IsCompressed() bool {
	return c.CompressedSize != c.UncompressedSize
}

// CreateReader creates a new reader for the chunk.
func (c ValvePakChunk) CreateReader(r io.ReaderAt) (io.Reader, error) {
	if c.IsCompressed() {
		return newLZHAMLazyReader(r, int64(c.Offset), int64(c.CompressedSize), int64(c.UncompressedSize)), nil
	} else {
		return io.NewSectionReader(r, int64(c.Offset), int64(c.CompressedSize)), nil
	}
}

type lzhamLazyReader struct {
	r   io.ReaderAt
	off int64
	csz int64
	dsz int64

	m sync.Mutex
	b []byte
	e error
	n uint64
}

func newLZHAMLazyReader(r io.ReaderAt, off, csz, dsz int64) io.Reader {
	return &lzhamLazyReader{r: r, off: off, csz: csz, dsz: dsz}
}

func (r *lzhamLazyReader) Read(b []byte) (n int, err error) {
	r.m.Lock()
	defer r.m.Unlock()

	if r.e != nil {
		return 0, r.e
	}
	if r.decompress(); r.e != nil {
		return 0, r.e
	}
	if r.n >= uint64(r.dsz) {
		r.b = nil
		r.e = io.EOF
		return 0, r.e
	}
	n = copy(b, r.b[r.n:])
	r.n += uint64(n)
	return
}

func (r *lzhamLazyReader) EnsureDecompressed() error {
	r.m.Lock()
	defer r.m.Unlock()

	return r.decompress()
}

func (r *lzhamLazyReader) decompress() error {
	if r.e != nil {
		return r.e
	}
	if r.b != nil {
		return nil
	}
	src := make([]byte, int(r.csz))
	if _, err := r.r.ReadAt(src, r.off); err != nil {
		r.e = fmt.Errorf("read chunk: %w", err)
		return r.e
	}
	dst := make([]byte, int(r.dsz))
	if n, _, _, err := tf2lzham.Decompress(dst, src); err != nil {
		r.e = fmt.Errorf("decompress chunk: %w", err)
		return r.e
	} else if n != len(dst) {
		r.e = fmt.Errorf("decompress chunk: %w", err)
		return r.e
	}
	r.b = dst
	return nil
}

// CreateReader creates a new reader for the raw data of the chunk.
func (c ValvePakChunk) CreateReaderRaw(r io.ReaderAt) (io.Reader, error) {
	return io.NewSectionReader(r, int64(c.Offset), int64(c.CompressedSize)), nil
}

// Deserialize parses a ValvePakChunk from r.
func (c *ValvePakChunk) Deserialize(r io.Reader) error {
	if err := binary.Read(r, binary.LittleEndian, &c.LoadFlags); err != nil {
		return fmt.Errorf("read chunk entry flags: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &c.TextureFlags); err != nil {
		return fmt.Errorf("read chunk texture flags: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &c.Offset); err != nil {
		return fmt.Errorf("read chunk archive offset: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &c.CompressedSize); err != nil {
		return fmt.Errorf("read chunk compressed size: %w", err)
	} else if c.CompressedSize == 0 {
		return fmt.Errorf("read chunk compressed size: must be non-zero")
	}
	if err := binary.Read(r, binary.LittleEndian, &c.UncompressedSize); err != nil {
		return fmt.Errorf("read chunk uncompressed size: %w", err)
	} else if c.UncompressedSize == 0 {
		return fmt.Errorf("read chunk uncompressed size: must be non-zero")
	}
	return nil
}

// Serialize writes an encoded ValvePakChunk to w.
func (c ValvePakChunk) Serialize(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, &c.LoadFlags); err != nil {
		return fmt.Errorf("write chunk entry flags: %w", err)
	}
	if err := binary.Write(w, binary.LittleEndian, c.TextureFlags); err != nil {
		return fmt.Errorf("write chunk texture flags: %w", err)
	}
	if err := binary.Write(w, binary.LittleEndian, c.Offset); err != nil {
		return fmt.Errorf("write chunk archive offset: %w", err)
	}
	if c.CompressedSize == 0 {
		return fmt.Errorf("write chunk compressed size: must be non-zero")
	} else if err := binary.Write(w, binary.LittleEndian, c.CompressedSize); err != nil {
		return fmt.Errorf("write chunk compressed size: %w", err)
	}
	if c.UncompressedSize == 0 {
		return fmt.Errorf("write chunk uncompressed size: must be non-zero")
	} else if err := binary.Write(w, binary.LittleEndian, c.UncompressedSize); err != nil {
		return fmt.Errorf("write chunk uncompressed size: %w", err)
	}
	return nil
}
