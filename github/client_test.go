package github

import (
	"strings"
	"testing"
	"time"
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

	prs, errs := GetPRs(DemoRepos, Options{})
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

func TestDemoStackOnlyWhenAskedFor(t *testing.T) {
	DemoMode = true
	t.Cleanup(func() { DemoMode = false })

	layers := func(opts Options) []PR {
		prs, _ := GetPRs(DemoRepos, opts)
		var out []PR
		for _, pr := range prs {
			if pr.Stack != nil {
				out = append(out, pr)
			}
		}
		return out
	}
	if got := layers(Options{}); len(got) != 0 {
		t.Fatalf("without Options.Stacks no PR should carry a stack, got %d", len(got))
	}
	got := layers(Options{Stacks: true})
	if len(got) != 3 {
		t.Fatalf("expected the 3 layers of the demo stack, got %d", len(got))
	}
	for _, pr := range got {
		if pr.Repo != got[0].Repo || pr.Stack.Number != got[0].Stack.Number {
			t.Errorf("#%d is not in the same repo and stack as #%d", pr.Number, got[0].Number)
		}
	}
}

func TestStackFieldsAreOnlyQueriedWhenAskedFor(t *testing.T) {
	if q := buildPRQuery(false); strings.Contains(q, "stack") || strings.Contains(q, "%!") {
		t.Errorf("query without stacks should not mention them:\n%s", q)
	}
	if q := buildPRQuery(true); !strings.Contains(q, "stack { number size baseRefName }") || !strings.Contains(q, "stackEntry { position }") {
		t.Errorf("query with stacks is missing the stack fields:\n%s", q)
	}
}

func TestGetPRsNoRepos(t *testing.T) {
	prs, errs := GetPRs(nil, Options{})
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

	pr.Stack = &Stack{Number: 7, Size: 3, Position: 2, Base: "main"}
	if got, want := pr.FormatForSlack(), "`(+1/-2)` [#42](u) `2/3` Fix thing"; got != want {
		t.Errorf("stacked slack = %q, want %q", got, want)
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
		156: "review_required",
		154: "approved",
		158: "draft",
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

// The stack fields below follow GitHub's schema: stack is the stack as a
// whole, stackEntry this PR's place in it, and both are null outside one.
func TestParsePRsReadsStack(t *testing.T) {
	out := []byte(`{"data":{"repository":{"id":"R_1"},"search":{"nodes":[
		{"number":144,"title":"Layer","url":"u","state":"OPEN",
		 "updatedAt":"2026-11-24T07:57:42Z","commits":{"nodes":[]},
		 "stack":{"number":7,"size":3,"baseRefName":"main"},"stackEntry":{"position":2}},
		{"number":150,"title":"On its own","url":"u2","state":"OPEN",
		 "updatedAt":"2026-11-20T08:00:00Z","commits":{"nodes":[]},
		 "stack":null,"stackEntry":null}
	]}}}`)
	prs, err := parsePRs(out)
	if err != nil {
		t.Fatal(err)
	}
	want := Stack{Number: 7, Size: 3, Position: 2, Base: "main"}
	if s := prs[0].Stack; s == nil || *s != want {
		t.Errorf("layer stack = %+v, want %+v", s, want)
	}
	if prs[0].StackTag() != "2/3" {
		t.Errorf("layer tag = %q, want 2/3", prs[0].StackTag())
	}
	if prs[1].Stack != nil || prs[1].StackTag() != "" {
		t.Errorf("a PR outside a stack should have none: %+v", prs[1].Stack)
	}
}

func TestGetClosedSinceDemoIsNewestClosedFirst(t *testing.T) {
	DemoMode = true
	t.Cleanup(func() { DemoMode = false })
	prs, errs := GetClosedSince(DemoRepos, time.Now().Add(-7*24*time.Hour))
	if len(errs) != 0 || len(prs) != len(demoClosedAll()) {
		t.Fatalf("expected all %d demo history PRs, got %d (errs %v)", len(demoClosedAll()), len(prs), errs)
	}
	for i, pr := range prs {
		if s := pr.Status(); s != "merged" && s != "closed" {
			t.Errorf("#%d has status %q", pr.Number, s)
		}
		if i > 0 && prs[i-1].ClosedAt.Before(pr.ClosedAt) {
			t.Errorf("not sorted newest closed first at index %d", i)
		}
	}
}
