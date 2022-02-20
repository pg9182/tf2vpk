package tf2vpk

import (
	"fmt"
	"hash"
	"io"
)

type crc struct {
	crc uint32
}

func NewCRC() hash.Hash32 {
	return new(crc)
}

func (d *crc) Size() int { return 4 }

func (d *crc) BlockSize() int { return 1 }

func (c *crc) Write(b []byte) (n int, err error) {
	c.crc = crcUpdate(c.crc, b)
	return len(b), nil
}

func (c *crc) Reset() {
	c.crc = 0
}

func (c *crc) Sum32() uint32 {
	return c.crc
}

var crcTable = [...]uint32{
	0x00000000, 0x1db71064, 0x3b6e20c8, 0x26d930ac, 0x76dc4190, 0x6b6b51f4, 0x4db26158, 0x5005713c,
	0xedb88320, 0xf00f9344, 0xd6d6a3e8, 0xcb61b38c, 0x9b64c2b0, 0x86d3d2d4, 0xa00ae278, 0xbdbdf21c,
}

func crcUpdate(crc uint32, b []byte) uint32 {
	crc = ^crc
	for _, x := range b {
		b := uint32(x)
		crc = (crc >> 4) ^ crcTable[(crc&0xF)^(b&0xF)]
		crc = (crc >> 4) ^ crcTable[(crc&0xF)^(b>>4)]
	}
	return ^crc
}

func (d *crc) Sum(in []byte) []byte {
	s := d.Sum32()
	return append(in, byte(s>>24), byte(s>>16), byte(s>>8), byte(s))
}

type hashReader struct {
	r   io.Reader
	sz  uint64
	crc uint32
	h   hash.Hash32
	n   uint64
	err error
}

func newCRCReader(r io.Reader, sz uint64, crc uint32) io.Reader {
	return &hashReader{r, sz, crc, NewCRC(), 0, nil}
}

func (r *hashReader) Read(b []byte) (n int, err error) {
	if r.err != nil {
		return 0, r.err
	}
	n, err = r.r.Read(b)
	_, _ = r.h.Write(b[:n])
	r.n += uint64(n)
	if err == nil {
		return
	}
	if err == io.EOF {
		if r.n != r.sz {
			return 0, io.ErrUnexpectedEOF
		}
		if r.crc != 0 && r.h.Sum32() != r.crc {
			err = fmt.Errorf("crc mismatch: expected %08X, got %08X", r.crc, r.h.Sum32())
		}
	}
	r.err = err
	return
}
