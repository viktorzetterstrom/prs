package main

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// skillDoc is a Claude Code skill that teaches an agent the non-interactive
// commands. It ships inside the binary so `prs skill install` always matches
// the version of prs that is installed.
//
//go:embed skill/SKILL.md
var skillDoc string

// runSkill handles `prs skill` (print the skill) and `prs skill install`
// (write it where Claude Code finds personal skills). Installing again
// overwrites the file.
func runSkill(args []string, stdout io.Writer) error {
	switch {
	case len(args) == 0:
		_, err := io.WriteString(stdout, skillDoc)
		return err
	case len(args) == 1 && args[0] == "install":
		path, err := skillPath()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(skillDoc), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "installed %s\n", path)
		return nil
	default:
		return usageError(fmt.Sprintf("unknown skill command %q (want install)", args[0]))
	}
}

// skillPath is where Claude Code looks for a personal skill named prs:
// skills/prs/SKILL.md under $CLAUDE_CONFIG_DIR, which defaults to ~/.claude.
func skillPath() (string, error) {
	dir := os.Getenv("CLAUDE_CONFIG_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".claude")
	}
	return filepath.Join(dir, "skills", "prs", "SKILL.md"), nil
}

// refreshSkill rewrites an installed skill that differs from the one built
// into this binary, so upgrading prs upgrades the skill on the next run. It
// never installs a skill that is not there, and it ignores errors: a stale
// skill is not worth failing a command over.
func refreshSkill() {
	path, err := skillPath()
	if err != nil {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) == skillDoc {
		return
	}
	_ = os.WriteFile(path, []byte(skillDoc), 0o644)
}
