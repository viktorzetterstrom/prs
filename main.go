package main

import (
	"flag"
	"fmt"
	"github.com/viktorzetterstrom/prs/github"
	"github.com/viktorzetterstrom/prs/ui"
	"os"
)

func main() {
	lastWeek := flag.Bool("last-week", false, "Show all PRs (including closed) from the last week")
	flag.Parse()

	prs, err := github.GetPRs(*lastWeek)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching PRs: %v\n", err)
		fmt.Fprintf(os.Stderr, "Make sure you're in a git repository and have gh CLI installed and authenticated.\n")
		os.Exit(1)
	}

	if len(prs) == 0 {
		if *lastWeek {
			fmt.Println("No pull requests found from the last week.")
		} else {
			fmt.Println("No open pull requests found.")
		}
		os.Exit(0)
	}

	if err := ui.Run(prs); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
