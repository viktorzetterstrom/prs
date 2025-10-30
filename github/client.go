package github

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

type PR struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	URL       string `json:"url"`
}

func GetPRs() ([]PR, error) {
	cmd := exec.Command("gh", "pr", "list", "--search", "involves:@me", "--json", "number,title,additions,deletions,url")
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
