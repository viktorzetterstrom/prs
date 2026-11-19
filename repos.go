package main

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/viktorzetterstrom/prs/config"
	"github.com/viktorzetterstrom/prs/github"
)

// runRepos handles `prs repos [add|rm OWNER/REPO…]`. Adding a repo that is
// already tracked, or removing one that is not, is a no-op with a note on
// stderr, so the commands are safe to repeat.
func runRepos(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return listRepos(stdout, stderr)
	}

	sub, names := args[0], args[1:]
	if sub == "remove" {
		sub = "rm"
	}
	if sub != "add" && sub != "rm" {
		return usageError(fmt.Sprintf("unknown repos command %q (want add or rm)", args[0]))
	}
	if len(names) == 0 {
		return usageError(fmt.Sprintf("prs repos %s needs at least one OWNER/REPO", sub))
	}
	for _, n := range names {
		if !github.ValidRepo(n) {
			return usageError(fmt.Sprintf("expected OWNER/REPO, got %q", n))
		}
	}
	if github.DemoMode {
		return errors.New("demo mode never changes the config")
	}

	cfg, _, err := config.Load()
	if err != nil {
		return fmt.Errorf("could not read config: %w", err)
	}

	var done, skipped []string
	for _, n := range names {
		// GitHub names are case-insensitive, so Owner/Repo and owner/repo
		// are the same repo.
		same := func(r string) bool { return strings.EqualFold(r, n) }
		tracked := slices.ContainsFunc(cfg.Repos, same)
		switch {
		case sub == "add" && tracked:
			skipped = append(skipped, "already tracked: "+n)
		case sub == "add":
			cfg.Repos = append(cfg.Repos, n)
			done = append(done, "tracking "+n)
		case tracked:
			cfg.Repos = slices.DeleteFunc(cfg.Repos, same)
			done = append(done, "stopped tracking "+n)
		default:
			skipped = append(skipped, "not tracked: "+n)
		}
	}

	if len(done) > 0 {
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("could not save config: %w", err)
		}
	}
	for _, line := range done {
		fmt.Fprintln(stdout, line)
	}
	for _, line := range skipped {
		fmt.Fprintln(stderr, line)
	}
	return nil
}

// listRepos prints the tracked repos one per line. With none tracked it says
// on stderr what prs falls back to, so an empty stdout is not a mystery.
func listRepos(stdout, stderr io.Writer) error {
	if github.DemoMode {
		for _, r := range github.DemoRepos {
			fmt.Fprintln(stdout, r)
		}
		return nil
	}

	cfg, _, err := config.Load()
	if err != nil {
		return fmt.Errorf("could not read config: %w", err)
	}
	for _, r := range cfg.Repos {
		fmt.Fprintln(stdout, r)
	}
	if len(cfg.Repos) == 0 {
		if cur := github.CurrentRepo(); cur != "" {
			fmt.Fprintf(stderr, "no repos tracked; prs falls back to the current directory's repo, %s\n", cur)
		} else {
			fmt.Fprintln(stderr, "no repos tracked; add one with `prs repos add OWNER/REPO`")
		}
	}
	return nil
}
