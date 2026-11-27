package github

import "time"

// demoPRs returns canned data for the --demo flag. Each entry is designed to
// exercise a distinct branch of Status so the TUI shows every state.
// The set is split across DemoRepos by PR number parity so the multi-repo
// view interleaves rows from both. The three layers of the demo stack have
// even numbers, so they share a repo as a real stack must; they come back
// as plain PRs unless opts asks for stacks.
func demoPRs(repo string, opts Options) []PR {
	all := demoAll()
	var out []PR
	for i, pr := range all {
		if DemoRepos[pr.Number%len(DemoRepos)] != repo {
			continue
		}
		pr.UpdatedAt = time.Now().Add(-time.Duration(i) * time.Hour)
		if !opts.Stacks {
			pr.Stack = nil
		}
		out = append(out, pr)
	}
	return out
}

func demoAll() []PR {
	return []PR{
		{
			Number: 143, Title: "Waiting in the merge queue",
			Additions: 6, Deletions: 6,
			URL:   "https://github.com/example/repo/pull/143",
			State: "OPEN", Mergeable: "MERGEABLE", ReviewDecision: "APPROVED",
			CheckState: "SUCCESS", InMergeQueue: true,
		},
		{
			Number: 156, Title: "Retry failed webhooks from the queue",
			Additions: 120, Deletions: 8,
			URL:   "https://github.com/example/repo/pull/156",
			State: "OPEN", Mergeable: "MERGEABLE", ReviewDecision: "REVIEW_REQUIRED",
			CheckState: "SUCCESS",
			Stack:      &Stack{Number: 7, Size: 3, Position: 2, Base: "main"},
		},
		{
			Number: 154, Title: "Add a table for queued webhook retries",
			Additions: 40, Deletions: 2,
			URL:   "https://github.com/example/repo/pull/154",
			State: "OPEN", Mergeable: "MERGEABLE", ReviewDecision: "APPROVED",
			CheckState: "SUCCESS",
			Stack:      &Stack{Number: 7, Size: 3, Position: 1, Base: "main"},
		},
		{
			Number: 158, Title: "Show retries in the webhook log",
			Additions: 35, Deletions: 4,
			URL:   "https://github.com/example/repo/pull/158",
			State: "OPEN", IsDraft: true,
			Stack: &Stack{Number: 7, Size: 3, Position: 3, Base: "main"},
		},
		{
			Number: 142, Title: "Approved, ready to merge",
			Additions: 12, Deletions: 3,
			URL:   "https://github.com/example/repo/pull/142",
			State: "OPEN", Mergeable: "MERGEABLE", ReviewDecision: "APPROVED",
			CheckState: "SUCCESS",
		},
		{
			Number: 141, Title: "CI is failing here",
			Additions: 8, Deletions: 2,
			URL:   "https://github.com/example/repo/pull/141",
			State: "OPEN", Mergeable: "MERGEABLE",
			CheckState: "FAILURE",
		},
		{
			Number: 140, Title: "Reviewer requested changes",
			Additions: 30, Deletions: 12,
			URL:   "https://github.com/example/repo/pull/140",
			State: "OPEN", Mergeable: "MERGEABLE", ReviewDecision: "CHANGES_REQUESTED",
			CheckState: "SUCCESS",
		},
		{
			Number: 139, Title: "Waiting on a reviewer",
			Additions: 1, Deletions: 1,
			URL:   "https://github.com/example/repo/pull/139",
			State: "OPEN", Mergeable: "MERGEABLE", ReviewDecision: "REVIEW_REQUIRED",
		},
		{
			Number: 138, Title: "Conflicts with base branch",
			Additions: 5, Deletions: 1,
			URL:   "https://github.com/example/repo/pull/138",
			State: "OPEN", Mergeable: "CONFLICTING",
		},
		{
			Number: 137, Title: "Still drafting this one",
			Additions: 200, Deletions: 50,
			URL:   "https://github.com/example/repo/pull/137",
			State: "OPEN", IsDraft: true,
		},
		{
			Number: 136, Title: "Just opened, nothing flagged yet",
			Additions: 4, Deletions: 0,
			URL:   "https://github.com/example/repo/pull/136",
			State: "OPEN", Mergeable: "MERGEABLE",
		},
	}

}

// demoClosedPRs returns canned merged and closed PRs for the history tab,
// split across DemoRepos by PR number parity like demoPRs.
func demoClosedPRs(repo string) []PR {
	var out []PR
	for i, pr := range demoClosedAll() {
		if DemoRepos[pr.Number%len(DemoRepos)] != repo {
			continue
		}
		pr.ClosedAt = time.Now().Add(-time.Duration(2+i*29) * time.Hour)
		pr.UpdatedAt = pr.ClosedAt
		out = append(out, pr)
	}
	return out
}

func demoClosedAll() []PR {
	return []PR{
		{
			Number: 135, Title: "Merged this morning",
			Additions: 40, Deletions: 8,
			URL: "https://github.com/example/repo/pull/135", State: "MERGED",
		},
		{
			Number: 134, Title: "Closed without merging",
			Additions: 3, Deletions: 3,
			URL: "https://github.com/example/repo/pull/134", State: "CLOSED",
		},
		{
			Number: 133, Title: "Merged a couple of days ago",
			Additions: 120, Deletions: 64,
			URL: "https://github.com/example/repo/pull/133", State: "MERGED",
		},
		{
			Number: 132, Title: "Small fix that went in last week",
			Additions: 2, Deletions: 1,
			URL: "https://github.com/example/repo/pull/132", State: "MERGED",
		},
	}
}
