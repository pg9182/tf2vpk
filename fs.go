package tf2vpk

import (
	"fmt"
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

// PathToValvePakRef attempts to return a ValvePak from the provided path. It
// may or may not exist.
func PathToValvePakRef(filename, prefix string) (ValvePakRef, error) {
	path, fn := filepath.Split(filepath.FromSlash(filename))
	name, _, err := SplitName(fn, prefix)
	if err != nil {
		return ValvePakRef{}, err
	}
	return ValvePakRef{path, prefix, name}, nil
}

// ValvePakRef references a VPK from the filesystem.
type ValvePakRef struct {
	Path   string
	Prefix string
	Name   string
}

func (v ValvePakRef) Resolve(i ValvePakIndex) string {
	if v.Path == "" {
		v.Path = "."
	}
	if v.Name == "" {
		panic("vpk name is required")
	}
	var fn string
	if i == ValvePakIndexDir {
		fn = v.Prefix
	}
	fn += v.Name + "_" + i.String() + ".vpk"
	return filepath.Join(v.Path, fn)
}

func (v ValvePakRef) List() ([]string, error) {
	if v.Path == "" {
		v.Path = "."
	}
	if v.Name == "" {
		panic("vpk name is required")
	}
	ds, err := os.ReadDir(v.Path)
	if err != nil {
		return nil, err
	}
	var ns []string
	for _, d := range ds {
		// ensure it's a vpk file belonging to us
		if name, _, err := SplitName(d.Name(), v.Prefix); err == nil && name == v.Name {
			ns = append(ns, d.Name())
		}
	}
	return ns, nil
}
