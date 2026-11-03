package github

import "time"

// demoPRs returns canned data for the --demo flag. Each entry is designed to
// exercise a distinct branch of StatusEmoji so the TUI shows every state.
// The set is split across DemoRepos by PR number parity so the multi-repo
// view interleaves rows from both.
func demoPRs(repo string) []PR {
	all := demoAll()
	var out []PR
	for i, pr := range all {
		if DemoRepos[pr.Number%len(DemoRepos)] != repo {
			continue
		}
		pr.UpdatedAt = time.Now().Add(-time.Duration(i) * time.Hour)
		out = append(out, pr)
	}
	return out
}

func demoAll() []PR {
	return []PR{
		{
			Number: 142, Title: "Approved, ready to merge",
			Additions: 12, Deletions: 3,
			URL:   "https://github.com/example/repo/pull/142",
			State: "OPEN", Mergeable: "MERGEABLE", ReviewDecision: "APPROVED",
			StatusCheckRollup: []StatusCheck{{Conclusion: "SUCCESS", Status: "COMPLETED"}},
		},
		{
			Number: 141, Title: "CI is failing here",
			Additions: 8, Deletions: 2,
			URL:   "https://github.com/example/repo/pull/141",
			State: "OPEN", Mergeable: "MERGEABLE",
			StatusCheckRollup: []StatusCheck{{Conclusion: "FAILURE", Status: "COMPLETED"}},
		},
		{
			Number: 140, Title: "Reviewer requested changes",
			Additions: 30, Deletions: 12,
			URL:   "https://github.com/example/repo/pull/140",
			State: "OPEN", Mergeable: "MERGEABLE", ReviewDecision: "CHANGES_REQUESTED",
			StatusCheckRollup: []StatusCheck{{Conclusion: "SUCCESS", Status: "COMPLETED"}},
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
