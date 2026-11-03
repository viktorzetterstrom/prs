package ui

import (
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/viktorzetterstrom/prs/config"
	"github.com/viktorzetterstrom/prs/github"
)

func newTestModel(t *testing.T, repos []string, currentRepo string) model {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	return newModel(repos, currentRepo, false)
}

func newFirstRunModel(t *testing.T, repos []string, currentRepo string) model {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	return newModel(repos, currentRepo, true)
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEscape}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func send(m model, msgs ...tea.Msg) (model, tea.Cmd) {
	var cmd tea.Cmd
	for _, msg := range msgs {
		var next tea.Model
		next, cmd = m.Update(msg)
		m = next.(model)
	}
	return m, cmd
}

func typeString(m model, s string) model {
	for _, r := range s {
		m, _ = send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func TestNoReposShowsHint(t *testing.T) {
	m := newTestModel(t, nil, "")
	if !strings.Contains(m.View(), "No repos tracked yet. Press c to pick some.") {
		t.Fatalf("expected empty-state hint, got:\n%s", m.View())
	}
	if cmd := m.Init(); cmd == nil {
		t.Fatal("Init should still schedule the tick")
	}
}

func TestListShowsKeyHints(t *testing.T) {
	m := newTestModel(t, []string{"a/one"}, "")
	v := m.View()
	for _, want := range []string{"copy for Slack", "open", "refresh", "repos", "quit"} {
		if !strings.Contains(v, want) {
			t.Errorf("hint %q missing from list view:\n%s", want, v)
		}
	}
	if !strings.Contains(v, "Open PRs") {
		t.Errorf("header should show the title:\n%s", v)
	}
	if strings.Contains(v, "a/one") {
		t.Errorf("header should not list repo names:\n%s", v)
	}
}

func TestOpeningConfigRequestsSuggestionsOnce(t *testing.T) {
	m := newTestModel(t, []string{"sanalabs/sierra-platform"}, "sanalabs/prs")
	m, cmd := send(m, key("c"))
	if cmd == nil || !m.suggestLoading {
		t.Fatal("entering config should kick off a suggestion fetch")
	}
	v := m.View()
	if !strings.Contains(v, "Tracked repos") {
		t.Fatalf("expected config title, got:\n%s", v)
	}
	if !strings.Contains(v, "✓ sanalabs/sierra-platform") {
		t.Errorf("tracked repo should be checked:\n%s", v)
	}
	if !strings.Contains(v, "○ sanalabs/prs") || !strings.Contains(v, "Current directory") {
		t.Errorf("current repo should appear unchecked under its own heading:\n%s", v)
	}
	if !strings.Contains(v, "Saved") {
		t.Errorf("configured repos should sit under a Saved heading:\n%s", v)
	}
	if !strings.Contains(v, "Looking up repos") {
		t.Errorf("should show suggestion loading state:\n%s", v)
	}

	m, _ = send(m, suggestMsg{repos: []string{"sanalabs/sierra-platform", "sanalabs/sierra-config"}})
	v = m.View()
	if !strings.Contains(v, "○ sanalabs/sierra-config") || !strings.Contains(v, "Suggested") {
		t.Errorf("untracked suggestion should be listed unchecked under Suggested:\n%s", v)
	}
	if strings.Count(v, "sierra-platform") != 1 {
		t.Errorf("tracked repo that is also suggested must not be duplicated:\n%s", v)
	}

	// Re-entering config must not refetch.
	m, _ = send(m, key("esc"))
	m, cmd = send(m, key("c"))
	if cmd != nil || m.suggestLoading {
		t.Fatal("suggestions should be fetched once per session")
	}
}

func TestFirstRunOpensPickerAndSavesOnLeave(t *testing.T) {
	m := newFirstRunModel(t, []string{"sanalabs/prs"}, "sanalabs/prs")

	if cmd := m.Init(); cmd == nil {
		t.Fatal("first-run Init should fetch suggestions")
	}
	if !strings.Contains(m.View(), "Welcome!") {
		t.Fatalf("first run should show welcome text:\n%s", m.View())
	}
	if strings.Contains(m.View(), "Saved") {
		t.Fatalf("first run has nothing saved yet; the preselected repo belongs under Current directory:\n%s", m.View())
	}

	if _, found, _ := config.Load(); found {
		t.Fatal("config should not exist before the user leaves the picker")
	}
	m, cmd := send(m, key("esc"))
	if m.mode != modeList {
		t.Fatalf("expected list mode, got %v", m.mode)
	}
	cfg, found, err := config.Load()
	if err != nil || !found || len(cfg.Repos) != 1 || cfg.Repos[0] != "sanalabs/prs" {
		t.Fatalf("leaving the first-run picker should persist the preselected repo: %v %v %v", cfg, found, err)
	}
	if m.firstRun {
		t.Fatal("firstRun should clear after save")
	}
	if cmd == nil {
		t.Fatal("leaving the picker should start the PR fetch")
	}
}

func TestFirstRunWithNothingSelectedDoesNotSave(t *testing.T) {
	m := newFirstRunModel(t, nil, "")
	m, _ = send(m, key("esc"))
	if _, found, _ := config.Load(); found {
		t.Fatal("empty selection must not be persisted")
	}
	if !strings.Contains(m.View(), "Press c to pick some") {
		t.Fatalf("expected empty state:\n%s", m.View())
	}
}

func TestToggleCandidateAddsRepoAndSavesConfig(t *testing.T) {
	m := newTestModel(t, []string{"sanalabs/sierra-platform"}, "sanalabs/sierra-config")
	m, _ = send(m, key("c"), key("down"), key(" "))

	if len(m.repos) != 2 || m.repos[1] != "sanalabs/sierra-config" {
		t.Fatalf("expected candidate added, repos = %v", m.repos)
	}
	cfg, _, err := config.Load()
	if err != nil || len(cfg.Repos) != 2 {
		t.Fatalf("config not saved: %v %v", cfg, err)
	}
	if !m.cfgDirty || m.gen != 1 {
		t.Fatalf("expected dirty flag and bumped gen, got dirty=%v gen=%d", m.cfgDirty, m.gen)
	}
}

func TestTogglingKeepsRowOrder(t *testing.T) {
	m := newTestModel(t, []string{"a/one", "a/two"}, "a/cur")
	m, _ = send(m, key("c"))
	m, _ = send(m, suggestMsg{repos: []string{"a/sug"}})
	want := []string{"a/one", "a/two", "a/cur", "a/sug"}

	check := func(step string) {
		t.Helper()
		got := m.configRows()
		if !slices.Equal(got, want) {
			t.Fatalf("%s: rows = %v, want %v", step, got, want)
		}
	}
	check("initial")

	// Untrack the first row: it must stay in place, unchecked.
	m, _ = send(m, key(" "))
	check("after untrack a/one")
	if slices.Contains(m.repos, "a/one") {
		t.Fatal("a/one should be untracked")
	}

	// Track the suggestion at the bottom: it must stay at the bottom.
	m, _ = send(m, key("down"), key("down"), key("down"), key(" "))
	check("after track a/sug")
	if !slices.Equal(m.repos, []string{"a/two", "a/sug"}) {
		t.Fatalf("tracked repos should follow display order, got %v", m.repos)
	}

	// Re-track the first row: back to its original position, not appended.
	m.cfgCursor = 0
	m, _ = send(m, key(" "))
	check("after re-track a/one")
	if !slices.Equal(m.repos, []string{"a/one", "a/two", "a/sug"}) {
		t.Fatalf("re-tracked repo should keep display order, got %v", m.repos)
	}

	// Adding by name appends to the Saved section, ahead of the other groups.
	m, _ = send(m, key("a"))
	m = typeString(m, "a/new")
	m, _ = send(m, key("enter"))
	want = []string{"a/one", "a/two", "a/new", "a/cur", "a/sug"}
	check("after add a/new")
	if m.cfgCursor != 2 {
		t.Fatalf("cursor should land on the added repo, got %d", m.cfgCursor)
	}
	secs := m.sections()
	if len(secs) != 3 || secs[0].title != "Saved" || !slices.Equal(secs[0].repos, []string{"a/one", "a/two", "a/new"}) {
		t.Fatalf("unexpected sections: %+v", secs)
	}
}

func TestToggleTrackedRepoRemovesIt(t *testing.T) {
	m := newTestModel(t, []string{"a/one", "a/two"}, "")
	m, _ = send(m, key("c"), key(" "))
	if len(m.repos) != 1 || m.repos[0] != "a/two" {
		t.Fatalf("expected a/one removed, repos = %v", m.repos)
	}
	cfg, _, _ := config.Load()
	if len(cfg.Repos) != 1 || cfg.Repos[0] != "a/two" {
		t.Fatalf("config not updated: %v", cfg.Repos)
	}
}

func TestAddRepoViaInput(t *testing.T) {
	m := newTestModel(t, nil, "")
	m, _ = send(m, key("c"), key("a"))
	if m.mode != modeAddRepo {
		t.Fatalf("expected add mode, got %v", m.mode)
	}

	// Invalid input is rejected and keeps the prompt open.
	m = typeString(m, "nope")
	m, _ = send(m, key("enter"))
	if m.mode != modeAddRepo || m.cfgErr == nil {
		t.Fatalf("invalid repo should be rejected, mode=%v err=%v", m.mode, m.cfgErr)
	}
	if !strings.Contains(m.View(), "expected OWNER/REPO") {
		t.Fatalf("validation error not shown:\n%s", m.View())
	}

	// Fix it up and confirm.
	m.input.SetValue("")
	m = typeString(m, "sanalabs/sierra-config")
	m, _ = send(m, key("enter"))
	if m.mode != modeConfig {
		t.Fatalf("expected back in config mode, got %v", m.mode)
	}
	if len(m.repos) != 1 || m.repos[0] != "sanalabs/sierra-config" {
		t.Fatalf("repo not added: %v", m.repos)
	}
	if m.cfgErr != nil {
		t.Fatalf("unexpected save error: %v", m.cfgErr)
	}
}

func TestLeavingConfigAfterChangeRefetches(t *testing.T) {
	github.DemoMode = true
	t.Cleanup(func() { github.DemoMode = false })

	m := newTestModel(t, []string{"example/platform"}, "")
	m.view.loaded = true
	m.view.prs = []github.PR{{Number: 1, Repo: "example/platform"}}

	m, _ = send(m, key("c"), key("a"))
	m = typeString(m, "example/config")
	m, cmd := send(m, key("enter"), key("esc"))

	if m.mode != modeList {
		t.Fatalf("expected list mode after esc, got %v", m.mode)
	}
	if m.view.loaded {
		t.Fatal("view should be invalidated after repo change")
	}
	if cmd == nil {
		t.Fatal("leaving config after a change should trigger a fetch")
	}
	msg := cmd()
	rm, ok := msg.(refreshMsg)
	if !ok {
		t.Fatalf("expected refreshMsg, got %T", msg)
	}
	if rm.gen != m.gen {
		t.Fatalf("fetch gen %d != model gen %d", rm.gen, m.gen)
	}
	m, _ = send(m, rm)
	if !m.view.loaded || len(m.view.prs) == 0 {
		t.Fatal("refresh result for current gen should populate the view")
	}
	if !strings.Contains(m.View(), "platform") || !strings.Contains(m.View(), "config") {
		t.Fatalf("multi-repo view should show a heading per repo:\n%s", m.View())
	}
}

func TestStaleRefreshIsDropped(t *testing.T) {
	m := newTestModel(t, []string{"a/one"}, "")
	m.gen = 2
	m, _ = send(m, refreshMsg{gen: 1, prs: []github.PR{{Number: 1}}})
	if m.view.loaded || len(m.view.prs) != 0 {
		t.Fatal("stale refresh should be ignored")
	}
}

func TestPartialFailureKeepsResultsAndShowsError(t *testing.T) {
	m := newTestModel(t, []string{"a/one", "a/two"}, "")
	m, _ = send(m, refreshMsg{
		prs:  []github.PR{{Number: 7, Title: "ok", Repo: "a/one", UpdatedAt: time.Now()}},
		errs: []github.RepoError{{Repo: "a/two", Err: errorf("boom")}},
	})
	if !m.view.loaded || len(m.view.prs) != 1 {
		t.Fatalf("partial failure should still load successful repos: loaded=%v prs=%d", m.view.loaded, len(m.view.prs))
	}
	out := m.View()
	if !strings.Contains(out, "a/two: boom") {
		t.Fatalf("per-repo error should be shown:\n%s", out)
	}
	if !strings.Contains(out, "[#7]") {
		t.Fatalf("successful PR should be listed:\n%s", out)
	}
}

func TestAllReposFailingShowsErrorBody(t *testing.T) {
	m := newTestModel(t, []string{"a/one"}, "")
	m, _ = send(m, refreshMsg{errs: []github.RepoError{{Repo: "a/one", Err: errorf("nope")}}})
	if m.view.loaded {
		t.Fatal("view should not be marked loaded when every repo failed")
	}
	if !strings.Contains(m.View(), "Could not load PRs") {
		t.Fatalf("expected error body:\n%s", m.View())
	}
}

type strErr string

func (e strErr) Error() string { return string(e) }
func errorf(s string) error    { return strErr(s) }

func TestListGroupsByRepoWithHeadings(t *testing.T) {
	m := newTestModel(t, []string{"a/platform", "a/config", "a/empty"}, "")
	now := time.Now()
	m, _ = send(m, refreshMsg{prs: []github.PR{
		// GetPRs delivers newest-first across repos; grouping must regroup by
		// repo while keeping that recency order inside each group.
		{Number: 3, Title: "cfg newest", Repo: "a/config", UpdatedAt: now},
		{Number: 2, Title: "plat newer", Repo: "a/platform", UpdatedAt: now.Add(-time.Hour)},
		{Number: 1, Title: "plat older", Repo: "a/platform", UpdatedAt: now.Add(-2 * time.Hour)},
	}})

	v := m.View()
	for _, want := range []string{"platform", "config", "empty", "no open pull requests"} {
		if !strings.Contains(v, want) {
			t.Errorf("missing %q:\n%s", want, v)
		}
	}
	// Headings follow tracked order and PRs sit under their own heading.
	ip, ic, ie := strings.Index(v, "platform"), strings.Index(v, "config"), strings.Index(v, "empty")
	i1, i2, i3 := strings.Index(v, "[#1]"), strings.Index(v, "[#2]"), strings.Index(v, "[#3]")
	if !(ip < i2 && i2 < i1 && i1 < ic && ic < i3 && i3 < ie) {
		t.Fatalf("rows out of order:\n%s", v)
	}
	if strings.Contains(v, "platform plat") {
		t.Fatalf("per-row repo tag should be gone once grouped:\n%s", v)
	}

	// Cursor walks PRs only, skipping headings and empty notes.
	if pr, _ := m.selectedPR(); pr.Number != 2 {
		t.Fatalf("first selectable PR should be #2, got #%d", pr.Number)
	}
	m, _ = send(m, key("down"), key("down"), key("down"))
	if pr, _ := m.selectedPR(); pr.Number != 3 {
		t.Fatalf("cursor should stop on the last PR (#3), got #%d", pr.Number)
	}
	if !strings.Contains(m.View(), "➤ ") || strings.Count(m.View(), "➤ ") != 1 {
		t.Fatalf("exactly one row should be selected:\n%s", m.View())
	}
}

func TestSingleRepoListIsFlat(t *testing.T) {
	m := newTestModel(t, []string{"a/one"}, "")
	m, _ = send(m, refreshMsg{prs: []github.PR{{Number: 1, Title: "x", Repo: "a/one"}}})
	v := m.View()
	if strings.Contains(v, "one\n") && strings.Count(v, "one") > 1 {
		t.Fatalf("single repo should not get a heading:\n%s", v)
	}
	if !strings.Contains(v, "[#1]") {
		t.Fatalf("PR should be listed:\n%s", v)
	}
}

func TestScrollWindowKeepsSelectionVisible(t *testing.T) {
	m := newTestModel(t, []string{"a/one"}, "")
	m, _ = send(m, tea.WindowSizeMsg{Width: 80, Height: 14}) // window = 6 lines
	var prs []github.PR
	for i := 1; i <= 20; i++ {
		prs = append(prs, github.PR{Number: i, Title: "pr", Repo: "a/one"})
	}
	m, _ = send(m, refreshMsg{prs: prs})
	for i := 0; i < 19; i++ {
		m, _ = send(m, key("down"))
	}
	v := m.View()
	if !strings.Contains(v, "➤ ") || !strings.Contains(v, "[#20]") {
		t.Fatalf("last PR should be selected and visible after scrolling:\n%s", v)
	}
	if strings.Contains(v, "[#1]") {
		t.Fatalf("top of list should have scrolled out of view:\n%s", v)
	}
}
