package github

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

type PR struct {
	Number         int
	Title          string
	Additions      int
	Deletions      int
	URL            string
	State          string
	IsDraft        bool
	Mergeable      string
	ReviewDecision string
	UpdatedAt      time.Time
	// ClosedAt is when the PR was merged or closed; zero while it is open.
	ClosedAt time.Time

	// CheckState is GitHub's combined check result for the head commit:
	// SUCCESS, FAILURE, ERROR, PENDING or EXPECTED, or "" when no checks
	// ran. GitHub only counts the latest run of each check, so a run that
	// was cancelled because a newer one replaced it does not fail the PR.
	CheckState string

	// InMergeQueue is set while the PR waits in the repo's merge queue.
	InMergeQueue bool

	// Repo is the OWNER/REPO the PR belongs to. GetPRs stamps it after each
	// per-repo call.
	Repo string

	// Stack is set when the PR is a layer of a GitHub stack and the fetch
	// asked for stacks (Options.Stacks); nil otherwise.
	Stack *Stack
}

// Stack is where a PR sits in a GitHub stack: a chain of PRs where each
// targets the branch of the one below, and the bottom one targets Base.
type Stack struct {
	// Number identifies the stack within its repository.
	Number int
	// Size is how many PRs the stack has, the user's or not.
	Size int
	// Position is this PR's layer: 1 is the bottom, the one closest to Base.
	Position int
	// Base is the branch the stack merges into, usually main.
	Base string
}

// Options turns on optional parts of a fetch.
type Options struct {
	// Stacks asks GitHub which stack each PR belongs to. It is opt-in
	// because stacks are new: a GitHub that does not know the fields fails
	// the whole query.
	Stacks bool
}

// ShortRepo returns the repo name without its owner, for compact display.
func (pr PR) ShortRepo() string {
	if i := strings.LastIndex(pr.Repo, "/"); i >= 0 {
		return pr.Repo[i+1:]
	}
	return pr.Repo
}

// DemoMode short-circuits GetPRs to return canned data instead of shelling out
// to `gh`. Used by the --demo CLI flag to preview the TUI without a network call.
var DemoMode bool

// DemoRepos is the repo list used when DemoMode is on, so the multi-repo UI
// can be previewed without a config file.
var DemoRepos = []string{"example/platform", "example/config"}

// prQuery fetches the caller's PRs in one repo that match a search filter. It goes through the
// GraphQL API rather than `gh pr list` because only GraphQL has the merge
// queue flag and GitHub's own combined check result. The repository field
// is there so a repo that does not exist fails the query; search alone
// would just come back empty. The %s is where stackFields go when asked for.
const prQuery = `query($owner: String!, $name: String!, $q: String!) {
  repository(owner: $owner, name: $name) { id }
  search(query: $q, type: ISSUE, first: 100) {
    nodes {
      ... on PullRequest {
        number title additions deletions url state isDraft mergeable
        reviewDecision updatedAt closedAt isInMergeQueue
        commits(last: 1) { nodes { commit { statusCheckRollup { state } } } }%s
      }
    }
  }
}`

// stackFields ask which stack a PR belongs to and at what position. Both
// are null for a PR outside a stack.
const stackFields = `
        stack { number size baseRefName }
        stackEntry { position }`

// buildPRQuery is prQuery with or without stackFields.
func buildPRQuery(stacks bool) string {
	extra := ""
	if stacks {
		extra = stackFields
	}
	return fmt.Sprintf(prQuery, extra)
}

// prNode is one PullRequest in prQuery's response.
type prNode struct {
	Number         int       `json:"number"`
	Title          string    `json:"title"`
	Additions      int       `json:"additions"`
	Deletions      int       `json:"deletions"`
	URL            string    `json:"url"`
	State          string    `json:"state"`
	IsDraft        bool      `json:"isDraft"`
	Mergeable      string    `json:"mergeable"`
	ReviewDecision string    `json:"reviewDecision"`
	UpdatedAt      time.Time `json:"updatedAt"`
	ClosedAt       time.Time `json:"closedAt"`
	IsInMergeQueue bool      `json:"isInMergeQueue"`
	Commits        struct {
		Nodes []struct {
			Commit struct {
				StatusCheckRollup *struct {
					State string `json:"state"`
				} `json:"statusCheckRollup"`
			} `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`
	Stack *struct {
		Number      int    `json:"number"`
		Size        int    `json:"size"`
		BaseRefName string `json:"baseRefName"`
	} `json:"stack"`
	StackEntry *struct {
		Position int `json:"position"`
	} `json:"stackEntry"`
}

func (n prNode) pr() PR {
	pr := PR{
		Number:         n.Number,
		Title:          n.Title,
		Additions:      n.Additions,
		Deletions:      n.Deletions,
		URL:            n.URL,
		State:          n.State,
		IsDraft:        n.IsDraft,
		Mergeable:      n.Mergeable,
		ReviewDecision: n.ReviewDecision,
		UpdatedAt:      n.UpdatedAt,
		ClosedAt:       n.ClosedAt,
		InMergeQueue:   n.IsInMergeQueue,
	}
	if c := n.Commits.Nodes; len(c) > 0 && c[0].Commit.StatusCheckRollup != nil {
		pr.CheckState = c[0].Commit.StatusCheckRollup.State
	}
	if n.Stack != nil && n.StackEntry != nil {
		pr.Stack = &Stack{
			Number:   n.Stack.Number,
			Size:     n.Stack.Size,
			Position: n.StackEntry.Position,
			Base:     n.Stack.BaseRefName,
		}
	}
	return pr
}

// RepoError is a fetch failure scoped to one repository, so a single bad or
// unreachable repo does not hide results from the others.
type RepoError struct {
	Repo string
	Err  error
}

func (e RepoError) Error() string { return fmt.Sprintf("%s: %v", e.Repo, e.Err) }

// GetPRs fetches the caller's open PRs in every repo concurrently and returns
// them merged, most recently updated first, together with one error per repo
// that failed.
func GetPRs(repos []string, opts Options) ([]PR, []RepoError) {
	prs, errs := fetchEach(repos, func(repo string) ([]PR, error) {
		if DemoMode {
			return demoPRs(repo, opts), nil
		}
		return searchPRs(repo, "is:open", opts.Stacks)
	})
	sort.SliceStable(prs, func(a, b int) bool {
		return prs[a].UpdatedAt.After(prs[b].UpdatedAt)
	})
	return prs, errs
}

// GetClosedSince fetches the caller's PRs in every repo that were merged or
// closed at or after since, most recently closed first, together with one
// error per repo that failed.
func GetClosedSince(repos []string, since time.Time) ([]PR, []RepoError) {
	prs, errs := fetchEach(repos, func(repo string) ([]PR, error) {
		if DemoMode {
			return demoClosedPRs(repo), nil
		}
		return searchPRs(repo, "is:closed closed:>="+since.UTC().Format(time.RFC3339), false)
	})
	sort.SliceStable(prs, func(a, b int) bool {
		return prs[a].ClosedAt.After(prs[b].ClosedAt)
	})
	return prs, errs
}

// fetchEach runs fetch for every repo concurrently, stamps each PR with its
// repo, and returns the PRs in repo order with one error per repo that failed.
func fetchEach(repos []string, fetch func(repo string) ([]PR, error)) ([]PR, []RepoError) {
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
			prs, err := fetch(repo)
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
	return merged, errs
}

// searchPRs runs prQuery for the caller's PRs in repo that match filter, a
// GitHub search qualifier such as "is:open", asking for stacks when stacks
// is set.
func searchPRs(repo, filter string, stacks bool) ([]PR, error) {
	owner, name, _ := strings.Cut(repo, "/")
	output, err := runGH("api", "graphql",
		"-f", "owner="+owner,
		"-f", "name="+name,
		"-f", "q=repo:"+repo+" is:pr author:@me "+filter,
		"-f", "query="+buildPRQuery(stacks))
	if err != nil {
		return nil, err
	}
	return parsePRs(output)
}

func parsePRs(output []byte) ([]PR, error) {
	var resp struct {
		Data struct {
			Search struct {
				Nodes []prNode `json:"nodes"`
			} `json:"search"`
		} `json:"data"`
	}
	if err := json.Unmarshal(output, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse PR data: %w", err)
	}
	var prs []PR
	for _, n := range resp.Data.Search.Nodes {
		// The search is limited to PRs, but a node of another type would
		// decode as an empty object.
		if n.Number != 0 {
			prs = append(prs, n.pr())
		}
	}
	return prs, nil
}

// SuggestRepos returns the repositories the current user has recently opened
// PRs in, most recently active first. It backs the first-run repo picker.
func SuggestRepos() ([]string, error) {
	if DemoMode {
		return append(slices.Clone(DemoRepos), "example/archive", "example/docs"), nil
	}

	output, err := runGH("search", "prs", "--author=@me", "--sort=updated", "--limit=100", "--json", "repository")
	if err != nil {
		return nil, err
	}

	var rows []struct {
		Repository struct {
			NameWithOwner string `json:"nameWithOwner"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(output, &rows); err != nil {
		return nil, fmt.Errorf("failed to parse search results: %w", err)
	}

	var repos []string
	for _, r := range rows {
		if name := r.Repository.NameWithOwner; name != "" && !slices.Contains(repos, name) {
			repos = append(repos, name)
		}
	}
	return repos, nil
}

func runGH(args ...string) ([]byte, error) {
	output, err := exec.Command("gh", args...).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("gh command failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("failed to execute gh command: %w", err)
	}
	return output, nil
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

// FormatForSlack renders the PR as a Slack-flavoured markdown line. The
// link text is the bare PR number; the URL already identifies the repo. The
// link sits before the title rather than at the end, because a pasted line
// ending in "#123" makes Slack pop up its channel autocomplete. A layer of a
// stack starts with its StackTag.
func (pr PR) FormatForSlack() string {
	tag := ""
	if t := pr.StackTag(); t != "" {
		tag = t + " "
	}
	return fmt.Sprintf("%s`(+%d/-%d)` [#%d](%s) %s",
		tag,
		pr.Additions,
		pr.Deletions,
		pr.Number,
		pr.URL,
		pr.Title)
}

// StackSize adds up the additions and deletions of layers.
func StackSize(layers []PR) (additions, deletions int) {
	for _, pr := range layers {
		additions += pr.Additions
		deletions += pr.Deletions
	}
	return additions, deletions
}

// StackTitle is the text of the line that heads a stack's layers, in the
// list and in Slack: how many PRs the stack has.
func StackTitle(layers []PR) string {
	return fmt.Sprintf("Stack of %d", layers[0].Stack.Size)
}

// StackCorner starts the line heading a stack's layers: a corner whose line
// runs on past where the layers have their StackTree glyph and StackTag,
// so the header's size starts right above the layers' sizes.
func StackCorner(layers []PR) string {
	return "┌" + strings.Repeat("─", len(layers[0].StackTag())+1)
}

// StackTree is the glyph that joins layer i of n to the line heading their
// stack, which has StackCorner: "│" beside each layer, and "└" beside the
// last.
func StackTree(i, n int) string {
	if i == n-1 {
		return "└"
	}
	return "│"
}

// FormatStackForSlack renders layers of one stack, bottom first, the way the
// list shows them: a heading line with StackCorner, the layers' combined
// size and StackTitle, then each layer's own Slack line, joined by
// StackTree glyphs.
// Each glyph starts its line, so they stay in one column in Slack's
// proportional font.
func FormatStackForSlack(layers []PR) string {
	add, del := StackSize(layers)
	lines := []string{fmt.Sprintf("%s `(+%d/-%d)` %s", StackCorner(layers), add, del, StackTitle(layers))}
	for i, pr := range layers {
		lines = append(lines, StackTree(i, len(layers))+" "+pr.FormatForSlack())
	}
	return strings.Join(lines, "\n")
}

// StackTag is the PR's place in its stack, such as "2." for the second
// layer from the bottom, or "" when it is not in one. The list and the Slack
// line both start with it, so a stack's layers read as a numbered list under
// the header, which already says how many there are.
func (pr PR) StackTag() string {
	if pr.Stack == nil {
		return ""
	}
	return fmt.Sprintf("%d.", pr.Stack.Position)
}

// CIStatus folds CheckState into FAILURE / PENDING / SUCCESS / "". Empty
// means no checks ran.
func (pr PR) CIStatus() string {
	switch pr.CheckState {
	case "FAILURE", "ERROR":
		return "FAILURE"
	case "PENDING", "EXPECTED":
		return "PENDING"
	case "SUCCESS":
		return "SUCCESS"
	}
	return ""
}

// Status names what most needs attention on the PR, first match wins:
// merged, closed, draft, queued, conflicting, ci_failing,
// changes_requested, approved, review_required, and open when nothing is
// flagged. It is part
// of the `prs --json` output, so the labels are documented in skill/SKILL.md.
func (pr PR) Status() string {
	switch pr.State {
	case "MERGED":
		return "merged"
	case "CLOSED":
		return "closed"
	}
	// OPEN beyond this point.
	if pr.IsDraft {
		return "draft"
	}
	if pr.InMergeQueue {
		// GitHub merges it once the queue's own checks pass, and takes it
		// out of the queue if they fail.
		return "queued"
	}
	if pr.Mergeable == "CONFLICTING" {
		return "conflicting"
	}
	if pr.CIStatus() == "FAILURE" {
		return "ci_failing"
	}
	switch pr.ReviewDecision {
	case "CHANGES_REQUESTED":
		return "changes_requested"
	case "APPROVED":
		return "approved"
	case "REVIEW_REQUIRED":
		return "review_required"
	}
	return "open"
}
