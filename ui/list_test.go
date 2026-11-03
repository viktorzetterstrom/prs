package ui

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
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
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	case "home":
		return tea.KeyMsg{Type: tea.KeyHome}
	case "end":
		return tea.KeyMsg{Type: tea.KeyEnd}
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

// runForRefresh executes cmd, unwrapping tea.Batch, and returns the first
// refreshMsg it produces.
func runForRefresh(cmd tea.Cmd) (refreshMsg, bool) {
	if cmd == nil {
		return refreshMsg{}, false
	}
	switch msg := cmd().(type) {
	case refreshMsg:
		return msg, true
	case tea.BatchMsg:
		for _, c := range msg {
			if rm, ok := runForRefresh(c); ok {
				return rm, true
			}
		}
	}
	return refreshMsg{}, false
}

func typeString(m model, s string) model {
	for _, r := range s {
		m, _ = send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func TestNoReposShowsHint(t *testing.T) {
	m := newTestModel(t, nil, "")
	if !strings.Contains(m.View(), "no repos tracked yet. press c to pick some.") {
		t.Fatalf("expected empty-state hint, got:\n%s", m.View())
	}
	if cmd := m.Init(); cmd == nil {
		t.Fatal("Init should still schedule the tick")
	}
}

func TestListShowsKeyHints(t *testing.T) {
	m := newTestModel(t, []string{"a/one"}, "")
	v := m.View()
	for _, want := range []string{"copy", "open", "refresh", "config"} {
		if !strings.Contains(v, want) {
			t.Errorf("hint %q missing from list view:\n%s", want, v)
		}
	}
	if !strings.Contains(v, "open prs") {
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
	if !strings.Contains(v, "config") {
		t.Fatalf("expected config title, got:\n%s", v)
	}
	if !strings.Contains(v, "✓ sanalabs/sierra-platform") {
		t.Errorf("tracked repo should be checked:\n%s", v)
	}
	if !strings.Contains(v, "○ sanalabs/prs") || !strings.Contains(v, "current directory") {
		t.Errorf("current repo should appear unchecked under its own heading:\n%s", v)
	}
	if !strings.Contains(v, "saved") {
		t.Errorf("configured repos should sit under a Saved heading:\n%s", v)
	}
	if !strings.Contains(v, "suggested from your recent prs") {
		t.Errorf("suggested section heading should show while loading:\n%s", v)
	}
	if !strings.Contains(v, strings.TrimRight(m.spinner.View(), " ")) {
		t.Errorf("spinner should show under the suggested heading while loading:\n%s", v)
	}

	m, _ = send(m, suggestMsg{repos: []string{"sanalabs/sierra-platform", "sanalabs/sierra-config"}})
	v = m.View()
	if !strings.Contains(v, "○ sanalabs/sierra-config") || !strings.Contains(v, "suggested") {
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
	if strings.Contains(m.View(), "saved") {
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
	if !strings.Contains(m.View(), "press c to pick some") {
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
	if len(secs) != 3 || secs[0].title != "saved" || !slices.Equal(secs[0].repos, []string{"a/one", "a/two", "a/new"}) {
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
	if !strings.Contains(m.View(), "expected owner/repo") {
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
	rm, ok := runForRefresh(cmd)
	if !ok {
		t.Fatal("expected a refreshMsg from the leave-config command")
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
	if !strings.Contains(m.View(), "could not load prs") {
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

func TestDemoModeNeverWritesConfig(t *testing.T) {
	github.DemoMode = true
	t.Cleanup(func() { github.DemoMode = false })

	m := newTestModel(t, []string{"example/platform"}, "")
	m, _ = send(m, key("c"), key(" ")) // untrack
	m, _ = send(m, key("a"))
	m = typeString(m, "example/added")
	m, _ = send(m, key("enter"), key("esc"))

	if len(m.repos) != 1 || m.repos[0] != "example/added" {
		t.Fatalf("in-memory repo list should still change in demo mode, got %v", m.repos)
	}
	if _, found, _ := config.Load(); found {
		t.Fatal("demo mode must not create a config file")
	}

	// First-run flow in demo mode must not save either.
	m = newFirstRunModel(t, []string{"example/platform"}, "example/platform")
	m, _ = send(m, key("esc"))
	if _, found, _ := config.Load(); found {
		t.Fatal("first-run demo must not create a config file")
	}
	if m.firstRun {
		t.Fatal("firstRun should still clear so the picker does not loop")
	}
}

func TestListNavigationKeys(t *testing.T) {
	m := newTestModel(t, []string{"a/one"}, "")
	m, _ = send(m, tea.WindowSizeMsg{Width: 80, Height: 28}) // window 20, page 10
	var prs []github.PR
	for i := 1; i <= 30; i++ {
		prs = append(prs, github.PR{Number: i, Repo: "a/one"})
	}
	m, _ = send(m, refreshMsg{prs: prs})

	at := func(step string, want int) {
		t.Helper()
		if pr, _ := m.selectedPR(); pr.Number != want {
			t.Fatalf("%s: selected #%d, want #%d", step, pr.Number, want)
		}
	}
	m, _ = send(m, key("j"), key("j"), key("k"))
	at("j j k", 2)
	m, _ = send(m, key("G"))
	at("G", 30)
	m, _ = send(m, key("g"))
	at("g", 1)
	m, _ = send(m, key("end"))
	at("End", 30)
	m, _ = send(m, key("home"))
	at("Home", 1)
	m, _ = send(m, key("pgdown"))
	at("PgDn", 11)
	m, _ = send(m, tea.KeyMsg{Type: tea.KeyCtrlD})
	at("Ctrl+D", 21)
	m, _ = send(m, tea.KeyMsg{Type: tea.KeyCtrlU})
	at("Ctrl+U", 11)
	m, _ = send(m, key("down"), key("up"))
	at("↓ ↑", 11)
}

func TestPickerEnterConfirmsAndSpaceToggles(t *testing.T) {
	m := newTestModel(t, []string{"a/one", "a/two"}, "")
	m, _ = send(m, key("c"), key("enter"))
	if m.mode != modeList {
		t.Fatalf("enter on the picker should confirm and leave, mode=%v", m.mode)
	}
	if len(m.repos) != 2 {
		t.Fatalf("enter must not toggle, repos=%v", m.repos)
	}
	m, _ = send(m, key("c"), key("G"), key(" "))
	if len(m.repos) != 1 || m.repos[0] != "a/one" {
		t.Fatalf("G then space should untrack the last repo, repos=%v", m.repos)
	}
}

func TestSpaceDoesNotCopyInList(t *testing.T) {
	m := newTestModel(t, []string{"a/one"}, "")
	m, _ = send(m, refreshMsg{prs: []github.PR{{Number: 1, Repo: "a/one", URL: "u"}}})
	m, _ = send(m, key(" "))
	if m.copied {
		t.Fatal("space is reserved for toggling on the picker; copy is enter/y")
	}
}

func TestCopiedMessageDoesNotShiftLayout(t *testing.T) {
	m := newTestModel(t, []string{"a/one"}, "")
	m, _ = send(m, refreshMsg{prs: []github.PR{{Number: 1, Title: "x", Repo: "a/one", URL: "u"}}})
	before := m.View()
	m.copied = true
	after := m.View()
	if strings.Count(before, "\n") != strings.Count(after, "\n") {
		t.Fatalf("copied message changed the line count:\n--- before\n%s\n--- after\n%s", before, after)
	}
	if !strings.Contains(after, "copied to clipboard") {
		t.Fatal("copied message missing")
	}
	hintLine := func(v string) int {
		lines := strings.Split(v, "\n")
		for i, l := range lines {
			if strings.Contains(l, "copy") {
				return i
			}
		}
		return -1
	}
	if hintLine(before) != hintLine(after) {
		t.Fatalf("key legend moved from line %d to %d", hintLine(before), hintLine(after))
	}
}

func TestFirstRunTogglesAreSavedOnLeave(t *testing.T) {
	m := newFirstRunModel(t, nil, "")
	m, _ = send(m, suggestMsg{repos: []string{"a/one", "a/two"}})
	m, _ = send(m, key(" "))              // track a/one
	m, _ = send(m, key("down"), key(" ")) // track a/two
	if !m.firstRun {
		t.Fatal("firstRun should last for the whole first visit")
	}
	m, _ = send(m, key("esc"))
	if m.firstRun {
		t.Fatal("firstRun should clear once the picker is left with a selection")
	}
	cfg, found, _ := config.Load()
	if !found || !slices.Equal(cfg.Repos, []string{"a/one", "a/two"}) {
		t.Fatalf("selection should be saved: %v %v", cfg.Repos, found)
	}
}

func TestLoadingSkeletonShowsRepoHeadings(t *testing.T) {
	m := newTestModel(t, []string{"a/platform", "a/config"}, "")
	cmd := m.startFetch()
	if cmd == nil || !m.view.loading {
		t.Fatal("startFetch should mark the view loading")
	}
	v := m.View()
	for _, want := range []string{"platform", "config"} {
		if !strings.Contains(v, want) {
			t.Errorf("loading skeleton missing %q:\n%s", want, v)
		}
	}
	frame := strings.TrimRight(m.spinner.View(), " ")
	if strings.Count(v, frame) != 2 {
		t.Fatalf("expected one bare spinner per repo:\n%s", v)
	}
	if strings.Contains(v, "loading") {
		t.Fatalf("startup skeleton should show the spinner only, no text:\n%s", v)
	}

	// Spinner ticks animate while loading and stop once the fetch lands.
	m, tick := send(m, spinner.TickMsg{ID: m.spinner.ID()})
	if tick == nil {
		t.Fatal("spinner should keep ticking while loading")
	}
	m, _ = send(m, refreshMsg{prs: []github.PR{{Number: 1, Repo: "a/platform"}}})
	_, tick = send(m, spinner.TickMsg{ID: m.spinner.ID()})
	if tick != nil {
		t.Fatal("spinner should stop ticking once nothing is loading")
	}
}

func TestSingleRepoLoadingIsOneLine(t *testing.T) {
	m := newTestModel(t, []string{"a/one"}, "")
	m.startFetch()
	v := m.View()
	frame := strings.TrimRight(m.spinner.View(), " ")
	if strings.Count(v, frame) != 1 || strings.Contains(v, "\n  one\n") {
		t.Fatalf("single repo should show one spinner and no heading:\n%s", v)
	}
}

func TestFirstFrameShowsLoadingSkeleton(t *testing.T) {
	m := newTestModel(t, []string{"a/platform", "a/config"}, "")
	v := m.View()
	frame := strings.TrimRight(m.spinner.View(), " ")
	if !m.view.loading || strings.Count(v, frame) != 2 {
		t.Fatalf("first frame should show a spinner under each heading before Init's fetch lands:\n%s", v)
	}
	if strings.Contains(v, "no open pull requests") {
		t.Fatalf("idle empty state must not flash before the first fetch:\n%s", v)
	}
	rm, ok := runForRefresh(m.Init())
	if !ok {
		t.Fatal("Init should start the fetch")
	}
	m, _ = send(m, rm)
	if m.view.loading {
		t.Fatal("fetch result should clear loading")
	}
}

func TestFirstRunFirstFrameShowsSuggestedLoading(t *testing.T) {
	m := newFirstRunModel(t, []string{"a/prs"}, "a/prs")
	v := m.View()
	if !m.suggestLoading || !strings.Contains(v, "suggested from your recent prs") {
		t.Fatalf("first-run first frame should show the suggested heading loading:\n%s", v)
	}
	if !strings.Contains(v, strings.TrimRight(m.spinner.View(), " ")) {
		t.Fatalf("spinner missing under suggested heading:\n%s", v)
	}
	if m.Init() == nil {
		t.Fatal("Init should start the suggestion lookup")
	}
}
