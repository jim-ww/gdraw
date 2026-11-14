package main

import (
	"encoding/json"
	"os"
	"slices"
)

const maxRecent = 9

func loadRecent() []string {
	path, err := dataFile("recent.json")
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var recent []string
	if json.Unmarshal(data, &recent) != nil {
		return nil
	}
	return recent
}

// saveRecent persists the list. Failures are ignored: the list is only a
// convenience.
func saveRecent(recent []string) {
	path, err := dataFile("recent.json")
	if err != nil {
		return
	}
	if data, err := json.MarshalIndent(recent, "", "  "); err == nil {
		_ = writeFileAtomic(path, data)
	}
}

// addRecent moves path to the front of recent, capped at maxRecent.
func addRecent(recent []string, path string) []string {
	out := append([]string{path}, slices.DeleteFunc(slices.Clone(recent), func(p string) bool { return p == path })...)
	return out[:min(len(out), maxRecent)]
}

func (u *UI) remember(path string) {
	u.recent = addRecent(u.recent, path)
	saveRecent(u.recent)
}
