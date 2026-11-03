package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileIsEmptyConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Repos) != 0 {
		t.Fatalf("expected no repos, got %v", cfg.Repos)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	want := Config{Repos: []string{"sanalabs/sierra-platform", "sanalabs/sierra-config"}}
	if err := want.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "prs", "config.json")); err != nil {
		t.Fatalf("config file not written at expected path: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Repos) != 2 || got.Repos[0] != want.Repos[0] || got.Repos[1] != want.Repos[1] {
		t.Fatalf("round trip mismatch: got %v want %v", got.Repos, want.Repos)
	}
}

func TestPathFallsBackToHomeDotConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/tmp/fakehome")
	p, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if p != "/tmp/fakehome/.config/prs/config.json" {
		t.Fatalf("unexpected path %q", p)
	}
}
