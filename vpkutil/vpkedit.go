package vpkutil

import (
	"fmt"
	"io"
	"os"

	"github.com/pg9182/tf2vpk"
)

// UpdateDir edits the vpk dir in-place.
func UpdateDir(vpk tf2vpk.ValvePakRef, dryRun bool, fn func(*tf2vpk.ValvePakDir) error) error {
	openFlag := os.O_RDWR
	if dryRun {
		openFlag = os.O_RDONLY
	}

	f, err := os.OpenFile(vpk.Resolve(tf2vpk.ValvePakIndexDir), openFlag, 0)
	if err != nil {
		return fmt.Errorf("open vpk dir: %w", err)
	}
	defer f.Close()

	var root tf2vpk.ValvePakDir
	if err := root.Deserialize(f); err != nil {
		return fmt.Errorf("read vpk dir: %w", err)
	}

	origSize, err := root.ChunkOffset()
	if err != nil {
		return fmt.Errorf("compute vpk dir size: %w", err)
	}

	if off, err := f.Seek(0, io.SeekCurrent); err != nil {
		return fmt.Errorf("read vpk dir: %w", err)
	} else if origSize != uint32(off) {
		panic("wtf") // this is a bug in ChunkOffset if it happens
	}

	if err := fn(&root); err != nil {
		return err
	}

	newSize, err := root.ChunkOffset()
	if err != nil {
		return fmt.Errorf("compute vpk dir size: %w", err)
	}

	if !dryRun {
		if newSize == origSize {
			if _, err := f.Seek(0, io.SeekStart); err != nil {
				return fmt.Errorf("write vpk dir: overwrite dir: %w", err)
			}
			if err := root.Serialize(f); err != nil {
				return fmt.Errorf("write vpk dir: overwrite dir: %w", err)
			}
		} else {
			tf, err := os.CreateTemp(vpk.Path, ".vpk*")
			if err != nil {
				return fmt.Errorf("write vpk dir: create temp file: %w", err)
			}
			defer os.Remove(tf.Name())
			defer tf.Close()

			if _, err := io.Copy(tf, f); err != nil {
				return fmt.Errorf("write vpk dir: copy chunks to temp file: %w", err)
			}
			if err := f.Truncate(0); err != nil {
				return fmt.Errorf("write vpk dir: truncate dir: %w", err)
			}
			if _, err := f.Seek(0, io.SeekStart); err != nil {
				return fmt.Errorf("write vpk dir: write dir: %w", err)
			}
			if err := root.Serialize(f); err != nil {
				return fmt.Errorf("write vpk dir: write dir: %w", err)
			}
			if _, err := tf.Seek(0, io.SeekStart); err != nil {
				return fmt.Errorf("write vpk dir: copy chunks from temp file: %w", err)
			}
			if _, err := io.Copy(f, tf); err != nil {
				return fmt.Errorf("write vpk dir: copy chunks from temp file: %w", err)
			}
			if err := f.Close(); err != nil {
				return fmt.Errorf("write vpk dir: write dir: %w", err)
			}

			if err := tf.Truncate(0); err != nil {
				return fmt.Errorf("write vpk dir: truncate temp file: %w", err)
			}
		}
	}
	return nil
}
