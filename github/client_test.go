package github

import (
	"testing"
)

func TestValidRepo(t *testing.T) {
	cases := map[string]bool{
		"sanalabs/sierra-platform": true,
		"a/b":                      true,
		"sierra-platform":          false,
		"a/b/c":                    false,
		"/b":                       false,
		"a/":                       false,
		"a b/c":                    false,
		"":                         false,
	}
	for in, want := range cases {
		if got := ValidRepo(in); got != want {
			t.Errorf("ValidRepo(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestRemoteRepoRegex(t *testing.T) {
	cases := map[string]string{
		"git@github.com:sanalabs/sierra-platform.git": "sanalabs/sierra-platform",
		"https://github.com/sanalabs/sierra-config":   "sanalabs/sierra-config",
		"https://github.com/foo/bar.git":              "foo/bar",
		"https://github.com/foo/bar/":                 "foo/bar",
		"ssh://git@github.com/foo/bar.git":            "foo/bar",
		"https://gitlab.com/foo/bar.git":              "",
	}
	for in, want := range cases {
		got := ""
		if m := remoteRepoRe.FindStringSubmatch(in); m != nil {
			got = m[1]
		}
		if got != want {
			t.Errorf("remote %q -> %q, want %q", in, got, want)
		}
	}
}

func TestGetPRsDemoMergesAndSortsAcrossRepos(t *testing.T) {
	DemoMode = true
	t.Cleanup(func() { DemoMode = false })

	prs, errs := GetPRs(QueryActive, DemoRepos)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(prs) != len(demoAll(QueryActive)) {
		t.Fatalf("expected all %d demo PRs across repos, got %d", len(demoAll(QueryActive)), len(prs))
	}

	seen := map[string]bool{}
	for i, pr := range prs {
		if pr.Repo == "" {
			t.Errorf("PR #%d has no repo stamped", pr.Number)
		}
		seen[pr.Repo] = true
		if i > 0 && prs[i-1].UpdatedAt.Before(pr.UpdatedAt) {
			t.Errorf("PRs not sorted newest first at index %d", i)
		}
	}
	if len(seen) != 2 {
		t.Fatalf("expected PRs from both demo repos, got %v", seen)
	}
}

func TestGetPRsNoRepos(t *testing.T) {
	prs, errs := GetPRs(QueryActive, nil)
	if len(prs) != 0 || len(errs) != 0 {
		t.Fatalf("expected empty result, got %v / %v", prs, errs)
	}
}

func TestShortRepoAndSlackFormat(t *testing.T) {
	pr := PR{Number: 42, Title: "Fix thing", Additions: 1, Deletions: 2, URL: "u", Repo: "sanalabs/sierra-config"}
	if pr.ShortRepo() != "sierra-config" {
		t.Fatalf("ShortRepo = %q", pr.ShortRepo())
	}
	if got, want := pr.FormatForSlack(false), "`(+1/-2)` Fix thing [#42](u)"; got != want {
		t.Errorf("single-repo slack = %q, want %q", got, want)
	}
	if got, want := pr.FormatForSlack(true), "`(+1/-2)` Fix thing [sierra-config#42](u)"; got != want {
		t.Errorf("multi-repo slack = %q, want %q", got, want)
	}
}
