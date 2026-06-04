package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/viktorzetterstrom/prs/github"
	"github.com/viktorzetterstrom/prs/ui"
)

func main() {
	lastWeek := flag.Bool("last-week", false, "Start on the 'Last 7 days' tab")
	demo := flag.Bool("demo", false, "Populate with fake PRs that exercise every status emoji")
	flag.Parse()

	github.DemoMode = *demo

	initial := github.QueryActive
	if *lastWeek {
		initial = github.QueryLastWeek
	}

	if err := ui.Run(initial); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
