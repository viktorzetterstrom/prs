package github

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

type PR struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	URL       string `json:"url"`
	State     string `json:"state"`
}

func GetPRs(lastWeek bool) ([]PR, error) {
	var searchQuery string
	var args []string

	if lastWeek {
		oneWeekAgo := time.Now().AddDate(0, 0, -7).Format("2006-01-02")
		searchQuery = fmt.Sprintf("author:@me updated:>%s", oneWeekAgo)

		args = []string{"pr", "list", "--state", "all", "--search", searchQuery, "--json", "number,title,additions,deletions,url,state"}
	} else {
		args = []string{"pr", "list", "--search", "author:@me", "--json", "number,title,additions,deletions,url,state"}
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
	title := pr.Title

	return fmt.Sprintf("`(+%d/-%d)` %s [#%d](%s)",
		pr.Additions,
		pr.Deletions,
		title,
		pr.Number,
		pr.URL)
}

func (pr PR) StatusEmoji() string {
	switch pr.State {
	case "OPEN":
		return "🟢"
	case "MERGED":
		return "🟣"
	case "CLOSED":
		return "🔴"
	default:
		return ""
	}
}
