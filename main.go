package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/mattn/go-isatty"
	"github.com/viktorzetterstrom/prs/config"
	"github.com/viktorzetterstrom/prs/github"
	"github.com/viktorzetterstrom/prs/ui"
)

const usage = `prs shows your open GitHub pull requests across the repos you track.

Usage:
  prs                        interactive list (a plain list when stdout is not a terminal)
  prs --json                 print the list as JSON and exit
  prs repos                  list tracked repos
  prs repos add OWNER/REPO…  start tracking repos
  prs repos rm OWNER/REPO…   stop tracking repos
  prs skill                  print a Claude Code skill that teaches an agent to use prs
  prs skill install          install that skill as ~/.claude/skills/prs/SKILL.md

Flags:
`

// usageError is a malformed command line. main prints it with a pointer to
// --help and exits 2 instead of 1.
type usageError string

func (e usageError) Error() string { return string(e) }

func main() {
	fd := os.Stdout.Fd()
	interactive := isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)

	err := run(os.Args[1:], os.Stdout, os.Stderr, interactive)
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	if errors.As(err, new(usageError)) {
		fmt.Fprintln(os.Stderr, "Run 'prs --help' for usage.")
		os.Exit(2)
	}
	os.Exit(1)
}

// run is main without the process plumbing. interactive reports whether
// stdout is a terminal; without one the list is printed instead of opening
// the TUI, which could not start anyway.
func run(args []string, stdout, stderr io.Writer, interactive bool) error {
	fs := flag.NewFlagSet("prs", flag.ExitOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), usage)
		fs.PrintDefaults()
	}
	demo := fs.Bool("demo", false, "Populate with fake PRs from fake repos; never touches the config")
	reset := fs.Bool("reset", false, "Delete the saved config and start over with the repo picker")
	asJSON := fs.Bool("json", false, "Print the PR list as JSON and exit")
	fs.Parse(args) // ExitOnError: exits itself on a bad flag or --help.

	github.DemoMode = *demo

	if *reset {
		if err := config.Remove(); err != nil {
			return fmt.Errorf("could not remove config: %w", err)
		}
	}

	if cmd := fs.Args(); len(cmd) > 0 {
		switch cmd[0] {
		case "repos":
			return runRepos(cmd[1:], stdout, stderr)
		case "skill":
			return runSkill(cmd[1:], stdout)
		default:
			return usageError(fmt.Sprintf("unknown command %q", cmd[0]))
		}
	}

	currentRepo := ""
	if !github.DemoMode {
		currentRepo = github.CurrentRepo()
	}
	repos, configExists, err := resolveRepos(currentRepo)
	if err != nil {
		return err
	}
	if interactive && !*asJSON {
		return ui.Run(repos, currentRepo, !configExists)
	}
	return printPRs(repos, stdout, stderr, *asJSON)
}

// resolveRepos builds the initial tracked-repo list: the config file wins,
// otherwise fall back to the repo of the current directory. The bool reports
// whether a config file existed, which decides if the repo picker opens first.
func resolveRepos(currentRepo string) (repos []string, configExists bool, err error) {
	if github.DemoMode {
		return slices.Clone(github.DemoRepos), true, nil
	}
	cfg, found, err := config.Load()
	if err != nil {
		return nil, found, fmt.Errorf("could not read config: %w", err)
	}
	if len(cfg.Repos) > 0 {
		return cfg.Repos, true, nil
	}
	if currentRepo != "" {
		return []string{currentRepo}, found, nil
	}
	return nil, found, nil
}
