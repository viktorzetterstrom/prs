package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/viktorzetterstrom/prs/github"
)

// prJSON is one PR in the `prs --json` output. The fields are documented for
// agents in skill/SKILL.md; keep the two in step.
type prJSON struct {
	Repo      string    `json:"repo"`
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	Status    string    `json:"status"`
	Draft     bool      `json:"draft"`
	Review    string    `json:"review"`
	CI        string    `json:"ci"`
	Mergeable string    `json:"mergeable"`
	Additions int       `json:"additions"`
	Deletions int       `json:"deletions"`
	UpdatedAt time.Time `json:"updatedAt"`
	Slack     string    `json:"slack"`
	// Stack is only there for a layer of a GitHub stack, and only with
	// stack support turned on.
	Stack *stackJSON `json:"stack,omitempty"`
}

// stackJSON is where a PR sits in its GitHub stack.
type stackJSON struct {
	Number   int    `json:"number"`
	Size     int    `json:"size"`
	Position int    `json:"position"`
	Base     string `json:"base"`
}

type repoErrorJSON struct {
	Repo  string `json:"repo"`
	Error string `json:"error"`
}

// listJSON is the whole `prs --json` document. Every slice is non-nil so the
// output has [] rather than null.
type listJSON struct {
	Repos  []string        `json:"repos"`
	PRs    []prJSON        `json:"prs"`
	Errors []repoErrorJSON `json:"errors"`
}

// printPRs fetches the PRs once and prints them as JSON or as tab-separated
// lines. Repos that fail do not hide the others; they are reported (in the
// JSON, or on stderr) and turn the result into an error so the exit code
// says something went wrong.
func printPRs(repos []string, opts github.Options, stdout, stderr io.Writer, asJSON bool) error {
	if len(repos) == 0 {
		return errors.New("no repos tracked; add one with `prs repos add OWNER/REPO`")
	}
	prs, errs := github.GetPRs(repos, opts)

	if asJSON {
		if err := writeJSON(stdout, repos, prs, errs); err != nil {
			return err
		}
	} else {
		writePlain(stdout, prs)
		for _, e := range errs {
			fmt.Fprintf(stderr, "Error: %v\n", e)
		}
		if len(prs) == 0 && len(errs) == 0 {
			fmt.Fprintf(stderr, "no open prs in %s\n", strings.Join(repos, ", "))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%d of %d repos could not be fetched", len(errs), len(repos))
	}
	return nil
}

func writeJSON(w io.Writer, repos []string, prs []github.PR, errs []github.RepoError) error {
	out := listJSON{
		Repos:  repos,
		PRs:    make([]prJSON, 0, len(prs)),
		Errors: make([]repoErrorJSON, 0, len(errs)),
	}
	for _, pr := range prs {
		p := prJSON{
			Repo:      pr.Repo,
			Number:    pr.Number,
			Title:     pr.Title,
			URL:       pr.URL,
			Status:    pr.Status(),
			Draft:     pr.IsDraft,
			Review:    pr.ReviewDecision,
			CI:        pr.CIStatus(),
			Mergeable: pr.Mergeable,
			Additions: pr.Additions,
			Deletions: pr.Deletions,
			UpdatedAt: pr.UpdatedAt,
			Slack:     pr.FormatForSlack(),
		}
		if s := pr.Stack; s != nil {
			p.Stack = &stackJSON{Number: s.Number, Size: s.Size, Position: s.Position, Base: s.Base}
		}
		out.PRs = append(out.PRs, p)
	}
	for _, e := range errs {
		out.Errors = append(out.Errors, repoErrorJSON{Repo: e.Repo, Error: e.Err.Error()})
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	// Titles and Slack lines go out as typed, not with & < > as & etc.
	enc.SetEscapeHTML(false)
	return enc.Encode(out)
}

// writePlain prints one tab-separated line per PR: repo, number, status,
// size, title, URL.
func writePlain(w io.Writer, prs []github.PR) {
	for _, pr := range prs {
		fmt.Fprintf(w, "%s\t#%d\t%s\t+%d/-%d\t%s\t%s\n",
			pr.Repo, pr.Number, pr.Status(), pr.Additions, pr.Deletions, pr.Title, pr.URL)
	}
}
