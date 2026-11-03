// Package config persists the list of repositories prs tracks.
//
// The file lives at $XDG_CONFIG_HOME/prs/config.json, defaulting to
// ~/.config/prs/config.json (the same convention gh itself uses, on every
// platform). A missing file is not an error: it means "no repos configured",
// and the UI falls back to the repo of the current directory.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Config struct {
	Repos []string `json:"repos"`
}

func Path() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "prs", "config.json"), nil
}

// Load reads the config file. The bool reports whether the file existed; a
// missing file yields an empty Config, false and a nil error so callers can
// detect a first run without special-casing the error.
func Load() (Config, bool, error) {
	path, err := Path()
	if err != nil {
		return Config{}, false, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, false, nil
	}
	if err != nil {
		return Config{}, false, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, true, err
	}
	return cfg, true, nil
}

func (c Config) Save() error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
