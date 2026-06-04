package github

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

type StatusCheck struct {
	Conclusion string `json:"conclusion"`
	State      string `json:"state"`
	Status     string `json:"status"`
}

type PR struct {
	Number            int           `json:"number"`
	Title             string        `json:"title"`
	Additions         int           `json:"additions"`
	Deletions         int           `json:"deletions"`
	URL               string        `json:"url"`
	State             string        `json:"state"`
	IsDraft           bool          `json:"isDraft"`
	Mergeable         string        `json:"mergeable"`
	ReviewDecision    string        `json:"reviewDecision"`
	StatusCheckRollup []StatusCheck `json:"statusCheckRollup"`
}

type QueryKind int

const (
	QueryActive QueryKind = iota
	QueryActionable
	QueryLastWeek
)

func (k QueryKind) Label() string {
	switch k {
	case QueryActionable:
		return "Action needed"
	case QueryLastWeek:
		return "Last 7 days"
	default:
		return "Active"
	}
}

// DemoMode short-circuits GetPRs to return canned data instead of shelling out
// to `gh`. Used by the --demo CLI flag to preview the TUI without a network call.
var DemoMode bool

const jsonFields = "number,title,additions,deletions,url,state,isDraft,mergeable,reviewDecision,statusCheckRollup"

func GetPRs(kind QueryKind) ([]PR, error) {
	if DemoMode {
		return demoPRs(kind), nil
	}

	var args []string

	switch kind {
	case QueryLastWeek:
		oneWeekAgo := time.Now().AddDate(0, 0, -7).Format("2006-01-02")
		searchQuery := fmt.Sprintf("author:@me is:closed updated:>%s", oneWeekAgo)
		args = []string{"pr", "list", "--state", "all", "--search", searchQuery, "--json", jsonFields}
	case QueryActionable:
		// Open PRs in this repo that want something from you — review requests or mentions —
		// but exclude your own so they don't double up with the Active tab.
		searchQuery := "is:open -author:@me (review-requested:@me OR mentions:@me)"
		args = []string{"pr", "list", "--state", "all", "--search", searchQuery, "--json", jsonFields}
	default:
		args = []string{"pr", "list", "--search", "author:@me", "--json", jsonFields}
	}

	cmd := exec.Command("gh", args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("gh command failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("failed to execute gh command: %w", err)
	}

	var prs []PR
	if err := json.Unmarshal(output, &prs); err != nil {
		return nil, fmt.Errorf("failed to parse PR data: %w", err)
	}

	return prs, nil
}

func (pr PR) FormatForDisplay() string {
	return fmt.Sprintf("#%d (+%d/-%d) %s", pr.Number, pr.Additions, pr.Deletions, pr.Title)
}

func (pr PR) FormatForSlack() string {
	return fmt.Sprintf("`(+%d/-%d)` %s [#%d](%s)",
		pr.Additions,
		pr.Deletions,
		pr.Title,
		pr.Number,
		pr.URL)
}

// ciStatus rolls up StatusCheckRollup into FAILURE / PENDING / SUCCESS / "".
// Empty means no checks are configured.
func (pr PR) ciStatus() string {
	if len(pr.StatusCheckRollup) == 0 {
		return ""
	}
	pending := false
	for _, c := range pr.StatusCheckRollup {
		switch c.Conclusion {
		case "FAILURE", "TIMED_OUT", "CANCELLED", "ACTION_REQUIRED", "STARTUP_FAILURE":
			return "FAILURE"
		}
		switch c.State {
		case "FAILURE", "ERROR":
			return "FAILURE"
		case "PENDING", "EXPECTED":
			pending = true
		}
		// CheckRun: status != COMPLETED means still running
		if c.Conclusion == "" && c.Status != "" && c.Status != "COMPLETED" {
			pending = true
		}
	}
	if pending {
		return "PENDING"
	}
	return "SUCCESS"
}

func (pr PR) StatusEmoji() string {
	switch pr.State {
	case "MERGED":
		return "🟣"
	case "CLOSED":
		return "⚫"
	}
	// OPEN beyond this point.
	if pr.IsDraft {
		return "📝"
	}
	if pr.Mergeable == "CONFLICTING" {
		return "⚠️"
	}
	if pr.ciStatus() == "FAILURE" {
		return "❌"
	}
	switch pr.ReviewDecision {
	case "CHANGES_REQUESTED":
		return "💬"
	case "APPROVED":
		return "✅"
	case "REVIEW_REQUIRED":
		return "👀"
	}
	return "🟢"
}
