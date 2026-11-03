package github

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
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
	UpdatedAt         time.Time     `json:"updatedAt"`

	// Repo is the OWNER/REPO the PR belongs to. `gh pr list` does not include
	// it in its JSON output, so GetPRs stamps it after each per-repo call.
	Repo string `json:"-"`
}

// ShortRepo returns the repo name without its owner, for compact display.
func (pr PR) ShortRepo() string {
	if i := strings.LastIndex(pr.Repo, "/"); i >= 0 {
		return pr.Repo[i+1:]
	}
	return pr.Repo
}

type QueryKind int

const (
	QueryActive QueryKind = iota
	QueryLastWeek
)

func (k QueryKind) Label() string {
	switch k {
	case QueryLastWeek:
		return "Last 7 days"
	default:
		return "Active"
	}
}

// DemoMode short-circuits GetPRs to return canned data instead of shelling out
// to `gh`. Used by the --demo CLI flag to preview the TUI without a network call.
var DemoMode bool

// DemoRepos is the repo list used when DemoMode is on, so the multi-repo UI
// can be previewed without a config file.
var DemoRepos = []string{"example/platform", "example/config"}

const jsonFields = "number,title,additions,deletions,url,state,isDraft,mergeable,reviewDecision,statusCheckRollup,updatedAt"

// RepoError is a fetch failure scoped to one repository, so a single bad or
// unreachable repo does not hide results from the others.
type RepoError struct {
	Repo string
	Err  error
}

func (e RepoError) Error() string { return fmt.Sprintf("%s: %v", e.Repo, e.Err) }

// GetPRs fetches PRs for every repo concurrently and returns them merged,
// most recently updated first, together with one error per repo that failed.
func GetPRs(kind QueryKind, repos []string) ([]PR, []RepoError) {
	type result struct {
		prs []PR
		err error
	}
	results := make([]result, len(repos))

	var wg sync.WaitGroup
	for i, repo := range repos {
		wg.Add(1)
		go func(i int, repo string) {
			defer wg.Done()
			prs, err := getRepoPRs(kind, repo)
			for j := range prs {
				prs[j].Repo = repo
			}
			results[i] = result{prs: prs, err: err}
		}(i, repo)
	}
	wg.Wait()

	var merged []PR
	var errs []RepoError
	for i, r := range results {
		if r.err != nil {
			errs = append(errs, RepoError{Repo: repos[i], Err: r.err})
			continue
		}
		merged = append(merged, r.prs...)
	}
	sort.SliceStable(merged, func(a, b int) bool {
		return merged[a].UpdatedAt.After(merged[b].UpdatedAt)
	})
	return merged, errs
}

func getRepoPRs(kind QueryKind, repo string) ([]PR, error) {
	if DemoMode {
		return demoPRs(kind, repo), nil
	}

	args := []string{"pr", "list", "--repo", repo, "--json", jsonFields}
	switch kind {
	case QueryLastWeek:
		oneWeekAgo := time.Now().AddDate(0, 0, -7).Format("2006-01-02")
		searchQuery := fmt.Sprintf("author:@me is:closed updated:>%s", oneWeekAgo)
		args = append(args, "--state", "all", "--search", searchQuery)
	default:
		args = append(args, "--search", "author:@me")
	}

	output, err := exec.Command("gh", args...).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("gh command failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("failed to execute gh command: %w", err)
	}

	var prs []PR
	if err := json.Unmarshal(output, &prs); err != nil {
		return nil, fmt.Errorf("failed to parse PR data: %w", err)
	}
	return prs, nil
}

var remoteRepoRe = regexp.MustCompile(`github\.com[:/]([^/\s]+/[^/\s]+?)(?:\.git)?/?$`)

// CurrentRepo returns the OWNER/REPO of the git repository in the current
// directory, derived from the origin remote URL. It does no network call.
// Returns "" when not in a repo or the remote is not on GitHub.
func CurrentRepo() string {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	m := remoteRepoRe.FindStringSubmatch(strings.TrimSpace(string(out)))
	if m == nil {
		return ""
	}
	return m[1]
}

// ValidRepo reports whether s looks like OWNER/REPO.
func ValidRepo(s string) bool {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return false
	}
	for _, p := range parts {
		if p == "" || strings.ContainsAny(p, " \t:") {
			return false
		}
	}
	return true
}

func (pr PR) FormatForDisplay() string {
	return fmt.Sprintf("#%d (+%d/-%d) %s", pr.Number, pr.Additions, pr.Deletions, pr.Title)
}

// FormatForSlack renders the PR as a Slack-flavoured markdown line. When
// showRepo is set the link text is "repo#123" instead of "#123", since a bare
// number is ambiguous once more than one repo is tracked.
func (pr PR) FormatForSlack(showRepo bool) string {
	ref := fmt.Sprintf("#%d", pr.Number)
	if showRepo {
		ref = pr.ShortRepo() + ref
	}
	return fmt.Sprintf("`(+%d/-%d)` %s [%s](%s)",
		pr.Additions,
		pr.Deletions,
		pr.Title,
		ref,
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
