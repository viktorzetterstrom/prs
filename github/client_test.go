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

	prs, errs := GetPRs(DemoRepos)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(prs) != len(demoAll()) {
		t.Fatalf("expected all %d demo PRs across repos, got %d", len(demoAll()), len(prs))
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
	prs, errs := GetPRs(nil)
	if len(prs) != 0 || len(errs) != 0 {
		t.Fatalf("expected empty result, got %v / %v", prs, errs)
	}
}

func TestShortRepoAndSlackFormat(t *testing.T) {
	pr := PR{Number: 42, Title: "Fix thing", Additions: 1, Deletions: 2, URL: "u", Repo: "sanalabs/sierra-config"}
	if pr.ShortRepo() != "sierra-config" {
		t.Fatalf("ShortRepo = %q", pr.ShortRepo())
	}
	if got, want := pr.FormatForSlack(), "`(+1/-2)` [#42](u) Fix thing"; got != want {
		t.Errorf("slack = %q, want %q", got, want)
	}
}

func TestSuggestReposDemo(t *testing.T) {
	DemoMode = true
	t.Cleanup(func() { DemoMode = false })
	repos, err := SuggestRepos()
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) < len(DemoRepos)+1 {
		t.Fatalf("expected demo suggestions beyond the tracked set, got %v", repos)
	}
}

func TestStatusOfEachDemoPR(t *testing.T) {
	want := map[int]string{
		143: "queued",
		142: "approved",
		141: "ci_failing",
		140: "changes_requested",
		139: "review_required",
		138: "conflicting",
		137: "draft",
		136: "open",
	}
	for _, pr := range demoAll() {
		if got := pr.Status(); got != want[pr.Number] {
			t.Errorf("#%d status = %q, want %q", pr.Number, got, want[pr.Number])
		}
	}
}

func TestCIStatusUsesGitHubsCombinedState(t *testing.T) {
	cases := map[string]string{
		"SUCCESS":  "SUCCESS",
		"FAILURE":  "FAILURE",
		"ERROR":    "FAILURE",
		"PENDING":  "PENDING",
		"EXPECTED": "PENDING",
		"":         "",
	}
	for state, want := range cases {
		if got := (PR{CheckState: state}).CIStatus(); got != want {
			t.Errorf("CheckState %q: CIStatus = %q, want %q", state, got, want)
		}
	}
}

// The response below is trimmed from a real one for a PR in the merge queue
// whose head commit had cancelled, superseded check runs. GitHub's combined
// state ignores those, so the PR must come out queued with CI passing.
func TestParsePRsReadsQueueAndCheckState(t *testing.T) {
	out := []byte(`{"data":{"repository":{"id":"R_1"},"search":{"nodes":[
		{"number":33529,"title":"Queued","additions":10,"deletions":2,"url":"u","state":"OPEN",
		 "isDraft":false,"mergeable":"MERGEABLE","reviewDecision":"APPROVED",
		 "updatedAt":"2026-11-24T07:57:42Z","isInMergeQueue":true,
		 "commits":{"nodes":[{"commit":{"statusCheckRollup":{"state":"SUCCESS"}}}]}},
		{"number":2,"title":"No checks","additions":0,"deletions":0,"url":"u2","state":"OPEN",
		 "isDraft":true,"mergeable":"UNKNOWN","reviewDecision":null,
		 "updatedAt":"2026-11-20T08:00:00Z","isInMergeQueue":false,
		 "commits":{"nodes":[{"commit":{"statusCheckRollup":null}}]}},
		{}
	]}}}`)
	prs, err := parsePRs(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(prs) != 2 {
		t.Fatalf("expected 2 PRs (the empty node skipped), got %d: %+v", len(prs), prs)
	}
	if q := prs[0]; !q.InMergeQueue || q.CIStatus() != "SUCCESS" || q.Status() != "queued" {
		t.Errorf("queued PR: inQueue=%v ci=%q status=%q", q.InMergeQueue, q.CIStatus(), q.Status())
	}
	if d := prs[1]; d.CheckState != "" || d.ReviewDecision != "" || d.Status() != "draft" {
		t.Errorf("draft PR: checks=%q review=%q status=%q", d.CheckState, d.ReviewDecision, d.Status())
	}
}
