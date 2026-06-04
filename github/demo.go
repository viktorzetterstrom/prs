package github

// demoPRs returns canned data for the --demo flag. Each entry is designed to
// exercise a distinct branch of StatusEmoji so the TUI shows every state.
func demoPRs(kind QueryKind) []PR {
	open := []PR{
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

	if kind == QueryActive {
		return open
	}

	// Last 7 days: only terminal states — open PRs already live on the Active tab.
	return []PR{
		{
			Number: 135, Title: "Shipped earlier this week",
			Additions: 45, Deletions: 20,
			URL:   "https://github.com/example/repo/pull/135",
			State: "MERGED",
		},
		{
			Number: 134, Title: "Another one shipped yesterday",
			Additions: 8, Deletions: 4,
			URL:   "https://github.com/example/repo/pull/134",
			State: "MERGED",
		},
		{
			Number: 133, Title: "Abandoned experiment",
			Additions: 3, Deletions: 3,
			URL:   "https://github.com/example/repo/pull/133",
			State: "CLOSED",
		},
	}
}
