package tf2vpk

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Ext is the file extension of a VPK.
const Ext = ".vpk"

// JoinName generates a filename for a VPK.
func JoinName(prefix, name string, idx ValvePakIndex) (fn string) {
	if idx != ValvePakIndexEOF {
		if idx == ValvePakIndexDir {
			fn = prefix
		}
		fn += name + "_" + idx.String() + Ext
	}
	return
}

// SplitName is the inverse of JoinName.
func SplitName(fn, prefix string) (name string, idx ValvePakIndex, err error) {
	var ok bool

	// ensure it's a vpk
	if fn, ok = strings.CutSuffix(fn, Ext); !ok {
		return "", ValvePakIndexEOF, fmt.Errorf("split %q (prefix %q): does not have extension %s", fn, prefix, Ext)
	}

	// if not, find the suffix, parse it, and cut it off
	if i := strings.LastIndex(fn, "_"); i == -1 || i == len(fn)-1 {
		return "", ValvePakIndexEOF, fmt.Errorf("split %q (prefix %q): vpk block does not have an index suffix", fn, prefix)
	} else {
		if idxStr := fn[i+1:]; idxStr == ValvePakIndexDir.String() {
			idx = ValvePakIndexDir
		} else if n, err := strconv.ParseUint(idxStr, 10, 16); err != nil {
			return "", ValvePakIndexEOF, fmt.Errorf("split %q (prefix %q): vpk block has an invalid index suffix: not a dir, and not an index: %w", fn, prefix, err)
		} else {
			idx = ValvePakIndex(n)
		}
		fn = fn[:i]
	}

	// if it's a vpk dir, ensure it has the prefix, and cut the prefix too
	if idx == ValvePakIndexDir {
		if fn, ok = strings.CutPrefix(fn, prefix); !ok {
			return "", ValvePakIndexEOF, fmt.Errorf("split %q (prefix %q): vpk dir index does not have expected prefix", fn, prefix)
		}
	}

	// the remaining text is the name
	name = fn
	return name, idx, nil
}

// ValvePak provides access to VPK files.
type ValvePak interface {
	// Open opens a reader for the provided index. It may also implement
	// io.Closer (this should be checked by the caller).
	Open(ValvePakIndex) (io.ReaderAt, error)

	// Create opens a writer writing to the provided index, truncating it if it
	// exists. It may also implement io.Closer (this should be checked by the
	// caller).
	Create(ValvePakIndex) (io.Writer, error)

	// Delete deletes all files associated with the VPK. If no files exist, nil
	// is returned.
	Delete() error
}

// VPK returns a ValvePak reading the provided path/prefix/name. It may or may
// not exist.
func VPK(path, prefix, name string) ValvePak {
	if path == "" {
		path = "."
	} else {
		path = filepath.FromSlash(path)
	}
	if name == "" {
		panic("vpk name is required")
	}
	return vpk{
		Path:   path,
		Prefix: prefix,
		Name:   name,
	}
}

// VPKFromPath attempts to return a ValvePak from the provided path. It may or
// may not exist.
func VPKFromPath(filename, prefix string) (ValvePak, error) {
	path, fn := filepath.Split(filepath.FromSlash(filename))
	name, _, err := SplitName(fn, prefix)
	if err != nil {
		return nil, err
	}
	return VPK(path, prefix, name), nil
}

type vpk struct {
	Path   string
	Prefix string
	Name   string
}

func (v vpk) Resolve(i ValvePakIndex) string {
	var fn string
	if i == ValvePakIndexDir {
		fn = v.Prefix
	}
	fn += v.Name + "_" + i.String() + ".vpk"
	return filepath.Join(v.Path, fn)
}

func (v vpk) Open(i ValvePakIndex) (io.ReaderAt, error) {
	return os.Open(v.Resolve(i))
}

func (v vpk) Create(i ValvePakIndex) (io.Writer, error) {
	return os.Create(v.Resolve(i))
}

func (v vpk) Delete() error {
	ds, err := os.ReadDir(v.Path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			err = nil
		}
		return err
	}

	var errs []error
	for _, d := range ds {
		// ensure it's a vpk file belonging to us
		name, _, err := SplitName(d.Name(), v.Prefix)
		if err != nil || name != v.Name {
			continue
		}

		// try and remove it
		if err := os.Remove(filepath.Join(v.Path, d.Name())); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				err = nil
			}
			errs = append(errs, err)
		}
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("delete vpk: %w", err)
	}
	return nil
}
