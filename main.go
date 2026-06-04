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
	flag.Parse()

	initial := github.QueryActive
	if *lastWeek {
		initial = github.QueryLastWeek
	}

	if err := ui.Run(initial); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
