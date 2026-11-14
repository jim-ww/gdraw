package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// fileVersion is written into every saved project. It matches bdraw's
// format, so projects open in either program.
const fileVersion = 1

type fileFormat struct {
	Version int     `json:"version"`
	Edits   []*Edit `json:"edits"`
}

// LoadDocument reads a project file. A missing file yields an empty
// document that will be created on first save.
func LoadDocument(path string) (*Document, error) {
	d := NewDocument()
	d.Path = path
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return d, nil
	}
	if err != nil {
		return nil, err
	}
	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if f.Version > fileVersion {
		return nil, fmt.Errorf("%s: file version %d is newer than supported %d", path, f.Version, fileVersion)
	}
	d.Edits = f.Edits
	bakeFills(d.Edits)
	for _, e := range d.Edits {
		d.nextID = max(d.nextID, e.ID)
	}
	return d, nil
}

func marshalDocument(d *Document) ([]byte, error) {
	return json.MarshalIndent(fileFormat{Version: fileVersion, Edits: d.Edits}, "", "  ")
}

// Save writes d as a project file to path.
func (d *Document) Save(path string) error {
	data, err := marshalDocument(d)
	if err != nil {
		return err
	}
	if err := writeFileAtomic(path, data); err != nil {
		return err
	}
	d.Path = path
	d.Dirty = false
	return nil
}

// writeFileAtomic writes data to a temp file next to path and renames it
// into place, so a crash never leaves a truncated file behind.
func writeFileAtomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// expandPath expands a leading "~/" to the home directory.
func expandPath(p string) string {
	p = strings.TrimSpace(p)
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, rest)
		}
	}
	return p
}

func hasExt(path, ext string) bool {
	return strings.EqualFold(filepath.Ext(path), ext)
}
