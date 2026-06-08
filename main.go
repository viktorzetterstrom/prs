package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/viktorzetterstrom/prs/github"
	"github.com/viktorzetterstrom/prs/ui"
)

func main() {
	demo := flag.Bool("demo", false, "Populate with fake PRs that exercise every status emoji")
	flag.Parse()

	github.DemoMode = *demo

	if err := ui.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
