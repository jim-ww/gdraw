package main

import (
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const autosaveInterval = 30 * time.Second

// autosaveFile is a recovery file: a project plus the path it belongs to.
type autosaveFile struct {
	fileFormat
	Path string `json:"path,omitempty"`
}

func autosavePath(d *Document) (string, error) {
	if d.autosaveID == "" {
		d.autosaveID = rand.Text()[:12]
	}
	return dataFile(filepath.Join("autosave", d.autosaveID+".json"))
}

// autosave writes every dirty document to its recovery file. Failures
// are ignored: autosave is a safety net, not worth interrupting for.
func (u *UI) autosave() {
	for _, d := range u.tabs {
		if !d.Dirty {
			continue
		}
		if len(d.Edits) == 0 {
			clearAutosave(d)
			continue
		}
		path, err := autosavePath(d)
		if err != nil {
			continue
		}
		data, err := json.Marshal(autosaveFile{fileFormat{fileVersion, d.Edits}, d.Path})
		if err != nil {
			continue
		}
		_ = writeFileAtomic(path, data)
	}
}

// clearAutosave removes d's recovery file, once its content is saved or
// deliberately discarded.
func clearAutosave(d *Document) {
	if d.autosaveID == "" {
		return
	}
	if path, err := autosavePath(d); err == nil {
		_ = os.Remove(path)
	}
}

// recoveryFiles lists recovery files left by earlier sessions.
func recoveryFiles() []string {
	dir, err := dataDir()
	if err != nil {
		return nil
	}
	files, _ := filepath.Glob(filepath.Join(dir, "autosave", "*.json"))
	return files
}

// loadRecovery reads a recovery file back into a dirty document that keeps
// writing to the same file.
func loadRecovery(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f autosaveFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	d := NewDocument()
	d.Path, d.Dirty = f.Path, true
	d.adopt(f.Edits)
	d.autosaveID = filepath.Base(path[:len(path)-len(filepath.Ext(path))])
	return d, nil
}

// restore opens every recovery file as a tab.
func (u *UI) restore() {
	files := recoveryFiles()
	for _, path := range files {
		d, err := loadRecovery(path)
		if err != nil {
			continue
		}
		u.addTab(d)
	}
	u.status = "restored " + plural(len(files), "recovery file")
}
