package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Config holds user preferences, read from config.json in configDir.
type Config struct {
	Palette      []string            `json:"palette,omitempty"`       // "#rrggbb" swatches
	DefaultColor string              `json:"default_color,omitempty"` // initial color
	DefaultSize  float64             `json:"default_size,omitempty"`  // initial brush size
	HideGrid     bool                `json:"hide_grid,omitempty"`
	Keys         map[string][]string `json:"keys,omitempty"` // action name to keys
}

// LoadConfig reads the config file at path, or the default location if
// path is empty. A missing default file is not an error.
func LoadConfig(path string) (Config, error) {
	var cfg Config
	explicit := path != ""
	if !explicit {
		dir, err := configDir()
		if err != nil {
			return cfg, nil
		}
		path = filepath.Join(dir, "config.json")
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) && !explicit {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing %s: %w", path, err)
	}
	return cfg, cfg.validate()
}

func (c *Config) validate() error {
	var errs []error
	for _, p := range c.Palette {
		if _, ok := parseHex(p); !ok {
			errs = append(errs, fmt.Errorf("palette: invalid color %q", p))
		}
	}
	if c.DefaultColor != "" {
		if _, ok := parseHex(c.DefaultColor); !ok {
			errs = append(errs, fmt.Errorf("default_color: invalid color %q", c.DefaultColor))
		}
	}
	if errs != nil {
		c.Palette, c.DefaultColor = nil, ""
	}
	return errors.Join(errs...)
}
