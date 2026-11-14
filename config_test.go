package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	write := func(name, s string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	cfg, err := LoadConfig(write("ok.json", `{"palette": ["#112233"], "default_size": 5, "keys": {"undo": ["u"]}}`))
	if err != nil || len(cfg.Palette) != 1 || cfg.DefaultSize != 5 || cfg.Keys["undo"][0] != "u" {
		t.Fatalf("LoadConfig = %+v, %v", cfg, err)
	}

	cfg, err = LoadConfig(write("bad.json", `{"palette": ["red"]}`))
	if err == nil || cfg.Palette != nil {
		t.Errorf("invalid palette: cfg = %+v, err = %v", cfg, err)
	}

	if _, err := LoadConfig(filepath.Join(dir, "missing.json")); err == nil {
		t.Error("explicit missing config: want error")
	}

	t.Setenv("XDG_CONFIG_HOME", dir)
	if _, err := LoadConfig(""); err != nil {
		t.Errorf("missing default config: %v", err)
	}
}
