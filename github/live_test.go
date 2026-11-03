package github

import (
	"os"
	"testing"
)

// TestGetPRsLive hits GitHub through gh. Opt in with PRS_LIVE_REPOS=owner/a,owner/b.
func TestGetPRsLive(t *testing.T) {
	spec := os.Getenv("PRS_LIVE_REPOS")
	if spec == "" {
		t.Skip("set PRS_LIVE_REPOS=owner/a,owner/b to run against GitHub")
	}
	repos := splitComma(spec)
	prs, errs := GetPRs(repos)
	for _, e := range errs {
		t.Errorf("%v", e)
	}
	t.Logf("%d open PRs", len(prs))
	for _, pr := range prs {
		t.Logf("  %s %-16s %s", pr.StatusEmoji(), pr.ShortRepo(), pr.FormatForDisplay())
	}

	suggested, err := SuggestRepos()
	if err != nil {
		t.Fatalf("SuggestRepos: %v", err)
	}
	t.Logf("suggested repos: %v", suggested)
}

func splitComma(s string) []string {
	var out []string
	for _, p := range splitOn(s, ',') {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func splitOn(s string, sep rune) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == sep {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	return append(out, cur)
}
