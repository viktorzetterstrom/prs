package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissingFileIsEmptyConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, found, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if found {
		t.Fatal("expected found=false for a missing file")
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

	got, found, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !found {
		t.Fatal("expected found=true after Save")
	}
	if len(got.Repos) != 2 || got.Repos[0] != want.Repos[0] || got.Repos[1] != want.Repos[1] {
		t.Fatalf("round trip mismatch: got %v want %v", got.Repos, want.Repos)
	}
}

func TestExperimentalSettingsRoundTripAndStayOutUntilOn(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "prs", "config.json")

	if err := (Config{Repos: []string{"a/b"}}).Save(); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); strings.Contains(string(data), "experimental") {
		t.Fatalf("an all-off experimental section should not be written:\n%s", data)
	}

	if err := (Config{Repos: []string{"a/b"}, Experimental: Experimental{Stacks: true}}).Save(); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); !strings.Contains(string(data), `"stacks": true`) {
		t.Fatalf("stack support should be saved:\n%s", data)
	}
	got, _, err := Load()
	if err != nil || !got.Experimental.Stacks {
		t.Fatalf("stack support did not round trip: %+v %v", got, err)
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

func TestRemove(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := Remove(); err != nil {
		t.Fatalf("Remove on a missing file should be a no-op, got %v", err)
	}
	if err := (Config{Repos: []string{"a/b"}}).Save(); err != nil {
		t.Fatal(err)
	}
	if err := Remove(); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, found, _ := Load(); found {
		t.Fatal("config should be gone after Remove")
	}
}
