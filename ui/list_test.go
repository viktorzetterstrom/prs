package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/viktorzetterstrom/prs/github"
)

func newTestModel(t *testing.T, repos []string, currentRepo string) model {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	stubClipboard(t, nil)
	return newModel(repos, currentRepo, false)
}

// stubClipboard replaces the clipboard for the rest of the test. Writes
// fail with err when it is non-nil; the returned func reports the last
// text written.
func stubClipboard(t *testing.T, err error) func() string {
	t.Helper()
	var last string
	orig := writeClipboard
	writeClipboard = func(s string) error {
		if err != nil {
			return err
		}
		last = s
		return nil
	}
	t.Cleanup(func() { writeClipboard = orig })
	return func() string { return last }
}

func newFirstRunModel(t *testing.T, repos []string, currentRepo string) model {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	stubClipboard(t, nil)
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
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
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
	for _, want := range []string{"switch", "copy", "open", "refresh", "config"} {
		if !strings.Contains(v, want) {
			t.Errorf("hint %q missing from list view:\n%s", want, v)
		}
	}
	if !strings.Contains(v, "open prs") || !strings.Contains(v, "last 7 days") {
		t.Errorf("header should show both tabs:\n%s", v)
	}
	if strings.Contains(v, "a/one") {
		t.Errorf("header should not list repo names:\n%s", v)
	}
}

func TestStaleRefreshIsDropped(t *testing.T) {
	m := newTestModel(t, []string{"a/one"}, "")
	m.gen = 2
	m, _ = send(m, refreshMsg{gen: 1, prs: []github.PR{{Number: 1}}})
	if m.tabs[tabOpen].loaded || len(m.tabs[tabOpen].prs) != 0 {
		t.Fatal("stale refresh should be ignored")
	}
}

func TestPartialFailureKeepsResultsAndShowsError(t *testing.T) {
	m := newTestModel(t, []string{"a/one", "a/two"}, "")
	m, _ = send(m, refreshMsg{
		prs:  []github.PR{{Number: 7, Title: "ok", Repo: "a/one", UpdatedAt: time.Now()}},
		errs: []github.RepoError{{Repo: "a/two", Err: errorf("boom")}},
	})
	if !m.tabs[tabOpen].loaded || len(m.tabs[tabOpen].prs) != 1 {
		t.Fatalf("partial failure should still load successful repos: loaded=%v prs=%d", m.tabs[tabOpen].loaded, len(m.tabs[tabOpen].prs))
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
	if m.tabs[tabOpen].loaded {
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

func TestLoadingSkeletonShowsRepoHeadings(t *testing.T) {
	m := newTestModel(t, []string{"a/platform", "a/config"}, "")
	cmd := m.startFetch()
	if cmd == nil || !m.tabs[tabOpen].loading {
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
	if !m.tabs[tabOpen].loading || strings.Count(v, frame) != 2 {
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
	if m.tabs[tabOpen].loading {
		t.Fatal("fetch result should clear loading")
	}
}

func TestCopyFailureIsShown(t *testing.T) {
	m := newTestModel(t, []string{"a/one"}, "")
	stubClipboard(t, errorf("no clipboard tool found"))
	m, _ = send(m, refreshMsg{prs: []github.PR{{Number: 1, Title: "x", Repo: "a/one", URL: "u"}}})
	m, _ = send(m, key("enter"))
	if m.copied {
		t.Fatal("a failed copy must not report success")
	}
	if v := m.View(); !strings.Contains(v, "could not copy: no clipboard tool found") {
		t.Fatalf("copy error not shown:\n%s", v)
	}
	m, _ = send(m, key("down"))
	if strings.Contains(m.View(), "could not copy") {
		t.Fatal("copy error should clear on the next key press")
	}
}

func TestCopyWritesSlackLineAndClearsError(t *testing.T) {
	m := newTestModel(t, []string{"a/one"}, "")
	written := stubClipboard(t, nil)
	pr := github.PR{Number: 1, Title: "x", Repo: "a/one", URL: "u"}
	m, _ = send(m, refreshMsg{prs: []github.PR{pr}})
	m.actionErr = errorf("could not open browser: nope")
	m, _ = send(m, key("y"))
	if !m.copied || m.actionErr != nil {
		t.Fatalf("copy should succeed and clear the old error: copied=%v err=%v", m.copied, m.actionErr)
	}
	if got := written(); got != pr.FormatForSlack() {
		t.Fatalf("clipboard = %q, want %q", got, pr.FormatForSlack())
	}
}

func TestHistoryTabLoadsOnFirstVisitAndKeepsOpenList(t *testing.T) {
	github.DemoMode = true
	t.Cleanup(func() { github.DemoMode = false })
	m := newTestModel(t, github.DemoRepos, "")
	open := github.PR{Number: 1, Title: "still open", Repo: github.DemoRepos[0], State: "OPEN"}
	m, _ = send(m, refreshMsg{tab: tabOpen, prs: []github.PR{open}})

	m, cmd := send(m, key("tab"))
	if m.tab != tabHistory {
		t.Fatal("tab should switch to the history tab")
	}
	msg, ok := runForRefresh(cmd)
	if !ok || msg.tab != tabHistory || len(msg.prs) == 0 {
		t.Fatalf("the first visit should load history: ok=%v msg=%+v", ok, msg)
	}
	for _, pr := range msg.prs {
		if s := pr.Status(); s != "merged" && s != "closed" {
			t.Errorf("history has #%d with status %q", pr.Number, s)
		}
	}
	m, _ = send(m, msg)

	v := m.View()
	if !strings.Contains(v, "merged") || !strings.Contains(v, "closed") || strings.Contains(v, "still open") {
		t.Fatalf("history tab should list merged and closed PRs only:\n%s", v)
	}
	if got := m.tabs[tabOpen].prs; len(got) != 1 || got[0].Number != 1 {
		t.Fatalf("loading history must not touch the open tab: %+v", got)
	}

	m, _ = send(m, key("left"))
	if m.tab != tabOpen || !strings.Contains(m.View(), "still open") {
		t.Fatalf("left should go back to the open tab:\n%s", m.View())
	}
	m, _ = send(m, key("left"))
	if m.tab != tabOpen {
		t.Fatal("left on the first tab should stay there")
	}
	m, cmd = send(m, key("right"))
	if m.tab != tabHistory || cmd != nil {
		t.Fatalf("right should show history again without refetching: tab=%v cmd=%v", m.tab, cmd != nil)
	}
	m, _ = send(m, key("right"))
	if m.tab != tabHistory {
		t.Fatal("right on the last tab should stay there")
	}
	m, _ = send(m, key("shift+tab"))
	if m.tab != tabOpen {
		t.Fatal("shift+tab should go to the previous tab")
	}
}

func TestEachTabKeepsItsCursor(t *testing.T) {
	m := newTestModel(t, []string{"a/one"}, "")
	prs := []github.PR{{Number: 1, Repo: "a/one"}, {Number: 2, Repo: "a/one"}, {Number: 3, Repo: "a/one"}}
	m, _ = send(m, refreshMsg{tab: tabOpen, prs: prs}, key("down"), key("down"))
	m, _ = send(m, key("tab"), refreshMsg{tab: tabHistory, prs: prs[:2]})
	if m.cur().cursor != 0 {
		t.Fatalf("history cursor should start at the top, got %d", m.cur().cursor)
	}
	m, _ = send(m, key("tab"))
	if pr, _ := m.selectedPR(); pr.Number != 3 {
		t.Fatalf("back on the open tab the selection should still be #3, got #%d", pr.Number)
	}
}

func TestEmptyHistorySaysSo(t *testing.T) {
	m := newTestModel(t, []string{"a/one", "a/two"}, "")
	m, _ = send(m, key("tab"), refreshMsg{tab: tabHistory, prs: []github.PR{{Number: 5, Repo: "a/one", State: "MERGED"}}})
	if v := m.View(); !strings.Contains(v, "nothing in the last 7 days") {
		t.Fatalf("a repo with no history should say so:\n%s", v)
	}
	m, _ = send(m, refreshMsg{tab: tabHistory})
	if v := m.View(); !strings.Contains(v, "nothing merged or closed in the last 7 days.") {
		t.Fatalf("an empty history should say so:\n%s", v)
	}
}

// stackLayer is an open PR that is layer pos of a three-layer stack into main.
func stackLayer(number, pos int, title string) github.PR {
	return github.PR{
		Number: number, Title: title, Repo: "a/one", State: "OPEN", URL: fmt.Sprintf("u%d", number),
		Additions: 10 * pos, Deletions: pos,
		Stack: &github.Stack{Number: 7, Size: 3, Position: pos, Base: "main"},
	}
}

func TestStackLayersAreGatheredUnderAHeader(t *testing.T) {
	m := newTestModel(t, []string{"a/one"}, "")
	written := stubClipboard(t, nil)
	middle := stackLayer(6, 2, "Middle layer")
	middle.ReviewDecision = "REVIEW_REQUIRED"
	bottom := stackLayer(5, 1, "Bottom layer")
	bottom.ReviewDecision = "APPROVED"
	m, _ = send(m, refreshMsg{prs: []github.PR{
		{Number: 9, Title: "Newest, alone", Repo: "a/one", State: "OPEN"},
		middle,
		{Number: 8, Title: "Older, alone", Repo: "a/one", State: "OPEN"},
		bottom,
	}})

	// The stack sits where its most recent layer did, bottom layer first.
	v := ansi.Strip(m.View())
	want := []string{
		"[#9] Newest, alone",
		"┌ (+30/-3) Stack of 3",
		"│ (+10/-1) [#5] 1/3 Bottom layer",
		"└ (+20/-2) [#6] 2/3 Middle layer",
		"[#8] Older, alone",
	}
	at := -1
	for _, w := range want {
		i := strings.Index(v, w)
		if i <= at {
			t.Fatalf("expected %q after the rows before it:\n%s", w, v)
		}
		at = i
	}
	// The header names the lowest layer holding the stack back.
	if strings.Contains(v, "into main") || !strings.Contains(v, "#6 needs review") {
		t.Fatalf("header should carry the blocking layer's label, and no branch:\n%s", v)
	}

	// The header is a row of its own: Enter there copies the whole stack.
	m, _ = send(m, key("down"))
	if _, ok := m.selectedPR(); ok {
		t.Fatal("the stack header is not a PR")
	}
	m, _ = send(m, key("enter"))
	if got, want := written(), github.FormatStackForSlack([]github.PR{bottom, middle}); got != want {
		t.Fatalf("enter on the header copied %q, want %q", got, want)
	}

	// On a layer, Enter copies that layer's line, position included.
	m, _ = send(m, key("down"), key("enter"))
	if got := written(); got != "`(+10/-1)` [#5](u5) `1/3` Bottom layer" {
		t.Fatalf("enter on the bottom layer copied %q", got)
	}
	m, _ = send(m, key("down"), key("down"))
	if pr, _ := m.selectedPR(); pr.Number != 8 {
		t.Fatalf("after the stack the cursor should reach #8, got #%d", pr.Number)
	}
}

func TestStackHeaderLabelWhenNothingBlocks(t *testing.T) {
	bottom, top := stackLayer(5, 1, "Bottom"), stackLayer(6, 2, "Top")
	bottom.ReviewDecision, top.ReviewDecision = "APPROVED", "APPROVED"
	top.InMergeQueue = true
	l, ok := stackLabel([]github.PR{bottom, top})
	if !ok || l.text != "queued" {
		t.Fatalf("with nothing blocking, the header should show the top layer's label, got %q", l.text)
	}
}

func TestStacklessPRsAreNotRegrouped(t *testing.T) {
	m := newTestModel(t, []string{"a/one"}, "")
	m, _ = send(m, refreshMsg{prs: []github.PR{{Number: 2, Repo: "a/one"}, {Number: 1, Repo: "a/one"}}})
	if v := m.View(); strings.ContainsAny(v, "┌│└") {
		t.Fatalf("without stacks there should be no tree:\n%s", v)
	}
	if len(m.cur().selectable) != 2 {
		t.Fatalf("each PR should be one selectable row, got %d", len(m.cur().selectable))
	}
}
