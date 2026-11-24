package ui

import (
	"slices"
	"strings"
	"testing"

	"github.com/viktorzetterstrom/prs/config"
	"github.com/viktorzetterstrom/prs/github"
)

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
