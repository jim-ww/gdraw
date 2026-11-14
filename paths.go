package main

import (
	"os"
	"path/filepath"
	"runtime"
)

const appName = "gdraw"

// configDir holds user-edited configuration.
func configDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, appName), nil
}

// dataDir holds program-managed state: recent files and autosaves. On
// Linux and BSD it follows the XDG data directory.
func dataDir() (string, error) {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return configDir()
	}
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, appName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", appName), nil
}

// dataFile returns the path of name in dataDir, creating the directory.
func dataFile(name string) (string, error) {
	dir, err := dataDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, name)
	return path, os.MkdirAll(filepath.Dir(path), 0o755)
}
