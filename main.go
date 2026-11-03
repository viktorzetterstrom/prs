package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/viktorzetterstrom/prs/config"
	"github.com/viktorzetterstrom/prs/github"
	"github.com/viktorzetterstrom/prs/ui"
)

func main() {
	demo := flag.Bool("demo", false, "Populate with fake PRs from fake repos; never touches the config")
	reset := flag.Bool("reset", false, "Delete the saved config and start over with the repo picker")
	flag.Parse()

	github.DemoMode = *demo

	if *reset {
		if err := config.Remove(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: could not remove config: %v\n", err)
			os.Exit(1)
		}
	}

	if err := ui.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
