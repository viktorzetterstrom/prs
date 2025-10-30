package main

import (
	"fmt"
	"os"
	"prs/github"
	"prs/ui"
)

func main() {
	prs, err := github.GetPRs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching PRs: %v\n", err)
		fmt.Fprintf(os.Stderr, "Make sure you're in a git repository and have gh CLI installed and authenticated.\n")
		os.Exit(1)
	}

	if len(prs) == 0 {
		fmt.Println("No open pull requests found.")
		os.Exit(0)
	}

	if err := ui.Run(prs); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
