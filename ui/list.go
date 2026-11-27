package ui

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/viktorzetterstrom/prs/config"
	"github.com/viktorzetterstrom/prs/github"
)

const refreshInterval = 30 * time.Second

// writeClipboard is clipboard.WriteAll, swapped out by tests so they never
// touch the real clipboard.
var writeClipboard = clipboard.WriteAll

var (
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("255"))
	numberStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	statsStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	emptyStyle        = lipgloss.NewStyle().PaddingLeft(4).Foreground(lipgloss.Color("241")).Italic(true)
	copiedStyle       = lipgloss.NewStyle().PaddingLeft(4).Foreground(lipgloss.Color("10")).Bold(true)
	timestampStyle    = lipgloss.NewStyle().PaddingLeft(4).Foreground(lipgloss.Color("241"))
	errorStyle        = lipgloss.NewStyle().PaddingLeft(4).Foreground(lipgloss.Color("9")).Bold(true)
	hintStyle         = lipgloss.NewStyle().PaddingLeft(4).Foreground(lipgloss.Color("241"))
	hintKeyStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	titleStyle        = lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1).Foreground(lipgloss.Color("255")).Background(lipgloss.Color("33")).Bold(true)
	tabStyle          = lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1).Foreground(lipgloss.Color("245"))
	sectionStyle      = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("245")).Bold(true)
	headerStyle       = lipgloss.NewStyle().PaddingLeft(2).PaddingBottom(1)
	checkedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	uncheckedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

// listRow is one line of the PR list: a repo heading, a PR, the header of a
// stack, or an "empty repo" note. PR rows and stack headers are selectable;
// prIndex indexes tabView.ordered.
type listRow struct {
	heading string
	prIndex int
	// stack holds a stack's layers, bottom first, on the row that heads them.
	stack []github.PR
	// indent goes in front of a PR row: stackIndent for a layer of a stack.
	indent string
	empty  bool
	spacer bool
}

type tickMsg time.Time

type refreshMsg struct {
	gen  int
	tab  tab
	prs  []github.PR
	errs []github.RepoError
}

type resetCopiedMsg struct{}

type openedMsg struct{ err error }

type viewState struct {
	prs         []github.PR
	loaded      bool
	loading     bool
	err         error
	lastUpdated time.Time
}

func (v *viewState) reset() {
	*v = viewState{}
}

// tab is one of the list's views. The open tab lists PRs still open; the
// history tab lists those merged or closed within historyWindow.
type tab int

const (
	tabOpen tab = iota
	tabHistory
	tabCount
)

const historyWindow = 7 * 24 * time.Hour

var tabTitles = [tabCount]string{"open prs", "last 7 days"}

// tabView is one tab's list: what was fetched for it, and ordered and rows,
// which regroup those PRs by tracked repo (in m.repos order) and lay out the
// lines over them. selectable lists the rows the cursor can land on, as
// indices into rows, and cursor indexes selectable. Each tab keeps its own,
// so switching back returns to the same place.
type tabView struct {
	viewState
	ordered    []github.PR
	rows       []listRow
	selectable []int
	cursor     int
}

type mode int

const (
	modeList mode = iota
	modeConfig
	modeAddRepo
)

type model struct {
	tabs   [tabCount]tabView
	tab    tab
	copied bool
	// actionErr is the last copy or open-in-browser failure, shown in the
	// footer until the next key press.
	actionErr error

	width  int
	height int

	// repos is the effective list of tracked repositories. It starts from the
	// config file, falling back to the current directory's repo when the
	// config is empty, and is written back to the config file whenever the
	// user changes it on the config screen.
	repos       []string
	currentRepo string
	// gen is bumped whenever repos change so in-flight fetches for the old
	// repo set are discarded when they land.
	gen int

	// firstRun is set when no config file existed at startup. The app then
	// opens on the repo picker and saves whatever is selected when the user
	// leaves it, so the picker does not reappear on the next launch.
	firstRun bool

	// experimental is the opt-in features from the config file, toggled on
	// the config screen and saved with the repo list.
	experimental config.Experimental

	mode      mode
	cfgCursor int
	cfgDirty  bool
	cfgErr    error
	input     textinput.Model
	spinner   spinner.Model
	// savedRepos is the "saved" section of the picker: the repos in the
	// config at startup plus anything added by name since. It is a frozen
	// snapshot so untracking a repo leaves its row in place.
	savedRepos []string

	// Repo suggestions come from the user's recent PR history and are fetched
	// once per session, the first time the config screen is shown.
	suggestions    []string
	suggestLoading bool
	suggestFetched bool
	suggestErr     error
}

func (m model) multiRepo() bool { return len(m.repos) > 1 }

// cur is the tab on screen.
func (m *model) cur() *tabView { return &m.tabs[m.tab] }

func openInBrowser(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		default:
			cmd = exec.Command("xdg-open", url)
		}
		return openedMsg{err: cmd.Start()}
	}
}

func fetchPRs(repos []string, gen int, t tab, opts github.Options) tea.Cmd {
	repos = slices.Clone(repos)
	return func() tea.Msg {
		var prs []github.PR
		var errs []github.RepoError
		if t == tabHistory {
			prs, errs = github.GetClosedSince(repos, time.Now().Add(-historyWindow))
		} else {
			prs, errs = github.GetPRs(repos, opts)
		}
		return refreshMsg{gen: gen, tab: t, prs: prs, errs: errs}
	}
}

// fetchOptions is what the experimental settings ask of a fetch.
func (m model) fetchOptions() github.Options {
	return github.Options{Stacks: m.experimental.Stacks}
}

// busy reports whether anything is in flight, which is when the spinner
// animates. Spinner ticks are dropped once nothing is loading.
func (m model) busy() bool {
	for _, v := range m.tabs {
		if v.loading {
			return true
		}
	}
	return m.suggestLoading
}

// startFetch loads the tab on screen.
func (m *model) startFetch() tea.Cmd {
	if len(m.repos) == 0 {
		return nil
	}
	m.cur().loading = true
	return tea.Batch(fetchPRs(m.repos, m.gen, m.tab, m.fetchOptions()), m.spinner.Tick)
}

// Init only returns commands. The loading flags they correspond to are set
// by newModel, because Init runs on a copy of the model and any state it
// changed would be lost.
func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) }),
	}
	if m.cur().loading {
		cmds = append(cmds, fetchPRs(m.repos, m.gen, m.tab, m.fetchOptions()), m.spinner.Tick)
	}
	if m.suggestLoading {
		cmds = append(cmds, fetchSuggestions, m.spinner.Tick)
	}
	return tea.Batch(cmds...)
}

func formatTimeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < 5*time.Second:
		return "updated just now"
	case d < time.Minute:
		return fmt.Sprintf("updated %ds ago", int(d.Seconds()))
	default:
		return fmt.Sprintf("updated %dm ago", int(d.Minutes()))
	}
}

// syncList regroups every tab's fetched PRs by repo.
func (m *model) syncList() {
	for i := range m.tabs {
		m.tabs[i].sync(m.repos)
	}
}

// sync regroups the fetched PRs by repo. With several repos tracked each
// repo gets a heading (in tracked order) and repos with nothing to show say
// so; with one repo the list is flat. Within either, a stack's layers are
// gathered under a header row (see groupStacks). The cursor is clamped to
// the new length.
func (v *tabView) sync(repos []string) {
	v.ordered = v.ordered[:0]
	v.rows = v.rows[:0]
	v.selectable = v.selectable[:0]

	if len(repos) <= 1 {
		v.addPRs(v.prs)
	} else {
		for i, repo := range repos {
			if i > 0 {
				v.rows = append(v.rows, listRow{spacer: true})
			}
			v.rows = append(v.rows, listRow{heading: github.PR{Repo: repo}.ShortRepo()})
			var inRepo []github.PR
			for _, pr := range v.prs {
				if pr.Repo == repo {
					inRepo = append(inRepo, pr)
				}
			}
			if len(inRepo) == 0 {
				v.rows = append(v.rows, listRow{empty: true})
			}
			v.addPRs(inRepo)
		}
	}

	if v.cursor >= len(v.selectable) {
		v.cursor = max(0, len(v.selectable)-1)
	}
}

// addPRs appends a row for each PR, with each stack's layers under a header
// row of its own.
func (v *tabView) addPRs(prs []github.PR) {
	for _, e := range groupStacks(prs) {
		if e.stack == nil {
			v.addPR(e.pr, "")
			continue
		}
		v.addSelectable(listRow{stack: e.stack})
		for _, pr := range e.stack {
			v.addPR(pr, stackIndent)
		}
	}
}

func (v *tabView) addPR(pr github.PR, indent string) {
	v.addSelectable(listRow{prIndex: len(v.ordered), indent: indent})
	v.ordered = append(v.ordered, pr)
}

func (v *tabView) addSelectable(row listRow) {
	v.selectable = append(v.selectable, len(v.rows))
	v.rows = append(v.rows, row)
}

// selectedRow is the row under the cursor: a PR or a stack's header.
func (m model) selectedRow() (listRow, bool) {
	v := m.cur()
	if v.cursor < 0 || v.cursor >= len(v.selectable) {
		return listRow{}, false
	}
	return v.rows[v.selectable[v.cursor]], true
}

// selectedPR is the PR under the cursor. On a stack's header there is none.
func (m model) selectedPR() (github.PR, bool) {
	row, ok := m.selectedRow()
	if !ok || row.stack != nil {
		return github.PR{}, false
	}
	return m.cur().ordered[row.prIndex], true
}

// selectedSlack is what Enter copies: the selected PR's Slack line, or on a
// stack's header the whole stack, the header's line and then each layer's.
func (m model) selectedSlack() (string, bool) {
	row, ok := m.selectedRow()
	switch {
	case !ok:
		return "", false
	case row.stack != nil:
		return github.FormatStackForSlack(row.stack), true
	}
	return m.cur().ordered[row.prIndex].FormatForSlack(), true
}

// selectedURL is what o opens: the selected PR, or on a stack's header its
// bottom layer, where review starts. Every layer's page maps the stack.
func (m model) selectedURL() (string, bool) {
	row, ok := m.selectedRow()
	switch {
	case !ok:
		return "", false
	case row.stack != nil:
		return row.stack[0].URL, true
	}
	return m.cur().ordered[row.prIndex].URL, true
}

// ensureLoaded starts loading the tab on screen unless it has been loaded or
// is loading already.
func (m *model) ensureLoaded() tea.Cmd {
	if v := m.cur(); v.loaded || v.loading {
		return nil
	}
	return m.startFetch()
}

// setTab switches to tab t, loading it the first time it is shown.
func (m *model) setTab(t tab) tea.Cmd {
	if t == m.tab {
		return nil
	}
	m.tab = t
	m.copied = false
	m.actionErr = nil
	return m.ensureLoaded()
}

// moveCursor applies the shared navigation keys (arrows, vim j/k, g/G,
// Home/End, PgUp/PgDn, Ctrl+U/Ctrl+D) to a cursor over n rows. It reports
// whether the key was a navigation key.
func moveCursor(key string, cursor, n, page int) (int, bool) {
	if n == 0 {
		return 0, isNavKey(key)
	}
	switch key {
	case "up", "k":
		cursor--
	case "down", "j":
		cursor++
	case "pgup", "ctrl+u":
		cursor -= page
	case "pgdown", "ctrl+d":
		cursor += page
	case "home", "g":
		cursor = 0
	case "end", "G":
		cursor = n - 1
	default:
		return cursor, false
	}
	return min(max(cursor, 0), n-1), true
}

func isNavKey(key string) bool {
	switch key {
	case "up", "k", "down", "j", "pgup", "ctrl+u", "pgdown", "ctrl+d", "home", "g", "end", "G":
		return true
	}
	return false
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		if !m.busy() {
			// Nothing loading: let this tick chain die.
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tickMsg:
		cmds := []tea.Cmd{tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })}
		// Only the tab on screen refreshes; the other catches up when shown.
		v := m.cur()
		if m.mode == modeList && v.loaded && !v.loading && time.Since(v.lastUpdated) >= refreshInterval {
			cmds = append(cmds, m.startFetch())
		}
		return m, tea.Batch(cmds...)

	case refreshMsg:
		if msg.gen != m.gen {
			// Result for a repo set the user has since changed; drop it.
			return m, nil
		}
		v := &m.tabs[msg.tab]
		v.loading = false
		v.lastUpdated = time.Now()
		allFailed := len(msg.errs) > 0 && len(msg.errs) == len(m.repos)
		if !allFailed {
			v.prs = msg.prs
			v.loaded = true
		}
		v.err = nil
		if len(msg.errs) > 0 {
			errs := make([]error, len(msg.errs))
			for i, e := range msg.errs {
				errs[i] = e
			}
			v.err = errors.Join(errs...)
		}
		m.syncList()
		return m, nil

	case suggestMsg:
		repoRows := len(m.configRows())
		m.suggestLoading = false
		m.suggestFetched = true
		m.suggestErr = msg.err
		m.suggestions = msg.repos
		// Suggestions land above the experimental section; keep a cursor
		// that was moved down there on the same setting. With no repo rows
		// yet the cursor has not moved, and stays at the top.
		if repoRows > 0 && m.cfgCursor >= repoRows {
			m.cfgCursor += len(m.configRows()) - repoRows
		}
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeConfig:
			return m.updateConfig(msg)
		case modeAddRepo:
			return m.updateAddRepo(msg)
		}

		key := msg.String()
		if cursor, ok := moveCursor(key, m.cur().cursor, len(m.cur().selectable), m.pageSize()); ok {
			m.cur().cursor = cursor
			m.copied = false
			m.actionErr = nil
			return m, nil
		}

		switch key {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "tab":
			return m, m.setTab((m.tab + 1) % tabCount)
		case "shift+tab":
			return m, m.setTab((m.tab + tabCount - 1) % tabCount)
		case "right", "l":
			return m, m.setTab(min(m.tab+1, tabCount-1))
		case "left", "h":
			return m, m.setTab(max(m.tab-1, 0))

		case "c":
			m.mode = modeConfig
			m.copied = false
			m.actionErr = nil
			m.cfgErr = nil
			m.cfgCursor = 0
			return m, m.ensureSuggestions()

		case "r":
			if !m.cur().loading {
				return m, m.startFetch()
			}
			return m, nil

		case "enter", "y":
			// Enter is the primary action; y is vim's "yank" for copy.
			if text, ok := m.selectedSlack(); ok {
				if err := writeClipboard(text); err != nil {
					m.copied = false
					m.actionErr = fmt.Errorf("could not copy: %w", err)
					return m, nil
				}
				m.copied = true
				m.actionErr = nil
				return m, tea.Tick(time.Millisecond*1500, func(t time.Time) tea.Msg {
					return resetCopiedMsg{}
				})
			}
			return m, nil

		case "o":
			if url, ok := m.selectedURL(); ok {
				m.actionErr = nil
				return m, openInBrowser(url)
			}
			return m, nil

		default:
			m.copied = false
			m.actionErr = nil
		}

	case resetCopiedMsg:
		m.copied = false
		return m, nil

	case openedMsg:
		m.actionErr = nil
		if msg.err != nil {
			m.actionErr = fmt.Errorf("could not open browser: %w", msg.err)
		}
		return m, nil
	}

	return m, nil
}

// hints renders a key legend like "↑/↓ move · enter copy · q quit". Only the
// primary key for each action is listed; vim-style aliases (j/k, g/G, y) and
// paging keys still work but are left to the README to keep the line short.
func hints(pairs ...string) string {
	parts := make([]string, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, hintKeyStyle.Render(pairs[i])+" "+pairs[i+1])
	}
	return hintStyle.Render(strings.Join(parts, " · "))
}

// renderHeader draws the tab titles, the one on screen highlighted.
func (m model) renderHeader() string {
	titles := make([]string, tabCount)
	for t := range tabCount {
		if t == m.tab {
			titles[t] = titleStyle.Render(tabTitles[t])
		} else {
			titles[t] = tabStyle.Render(tabTitles[t])
		}
	}
	return headerStyle.Render(strings.Join(titles, " "))
}

// emptyText is what a tab says when there is nothing to list, for the
// whole list (all) or under one repo's heading.
func (m model) emptyText(all bool) string {
	switch {
	case m.tab == tabHistory && all:
		return "nothing merged or closed in the last 7 days."
	case m.tab == tabHistory:
		return "nothing in the last 7 days"
	case all:
		return "no open pull requests."
	}
	return "no open pull requests"
}

func (m model) View() string {
	if m.mode != modeList {
		return m.viewConfig()
	}

	v := m.cur()

	var body string
	switch {
	case len(m.repos) == 0:
		body = itemStyle.Render("no repos tracked yet. press c to pick some.")
	case v.loading && !v.loaded:
		body = m.renderLoading()
	case v.err != nil && !v.loaded:
		body = errorStyle.Render(fmt.Sprintf("⚠ could not load prs:\n%v", indent(v.err.Error(), 2)))
	case len(v.prs) == 0:
		body = itemStyle.Render(m.emptyText(true))
	default:
		body = m.renderRows()
	}

	// The footer always occupies the same number of lines so transient
	// messages (copied, or a failed copy or open) never push the timestamp
	// or the key legend around. Persistent refresh errors are the one
	// exception: they are multi-line and stay until the next successful
	// fetch.
	var footer strings.Builder
	if v.err != nil && v.loaded {
		footer.WriteString("\n" + errorStyle.Render(fmt.Sprintf("⚠ refresh failed:\n%v", indent(v.err.Error(), 2))))
	}
	status := ""
	switch {
	case m.copied:
		status = copiedStyle.Render("✓ copied to clipboard!")
	case m.actionErr != nil:
		status = errorStyle.Render(fmt.Sprintf("⚠ %v", m.actionErr))
	}
	footer.WriteString("\n" + status)
	timestamp := ""
	switch {
	case v.loading && v.loaded:
		timestamp = timestampStyle.Render(m.spinner.View())
	case v.loaded:
		timestamp = timestampStyle.Render(formatTimeAgo(v.lastUpdated))
	}
	footer.WriteString("\n" + timestamp)
	footer.WriteString("\n\n" + hints("←/→", "switch", "↑/↓", "move", "enter", "copy", "o", "open", "r", "refresh", "c", "config"))

	return m.renderHeader() + "\n" + body + "\n" + footer.String()
}

// windowHeight is how many list lines fit: the terminal height minus the
// header, footer and key legend. Before the first WindowSizeMsg it is 16.
func (m model) windowHeight() int {
	if m.height > 0 {
		return max(3, m.height-8)
	}
	return 16
}

// pageSize is how far PgUp/PgDn move: half a window, like less/vim.
func (m model) pageSize() int { return max(1, m.windowHeight()/2) }

// renderLoading is the startup skeleton: the same headings the loaded list
// will have, each with a bare spinner beneath, so nothing jumps when the
// results arrive. With a single repo it is one spinner.
func (m model) renderLoading() string {
	line := itemStyle.Render(strings.TrimRight(m.spinner.View(), " "))
	if !m.multiRepo() {
		return line
	}
	var b strings.Builder
	for i, repo := range m.repos {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(sectionStyle.Render(github.PR{Repo: repo}.ShortRepo()))
		b.WriteString("\n")
		b.WriteString(line)
	}
	return b.String()
}

// renderRows draws the grouped list, scrolled so the selected row is visible.
func (m model) renderRows() string {
	// The layout spans every PR, not just the visible window, so the label
	// column does not shift while scrolling.
	v := m.cur()
	layout := newRowLayout(v.ordered, m.width)
	lines := make([]string, len(v.rows))
	selectedLine := 0
	if v.cursor < len(v.selectable) {
		selectedLine = v.selectable[v.cursor]
	}
	for i, row := range v.rows {
		selected := i == selectedLine && len(v.selectable) > 0
		switch {
		case row.spacer:
			lines[i] = ""
		case row.heading != "":
			lines[i] = sectionStyle.Render(row.heading)
		case row.empty:
			lines[i] = emptyStyle.Render(m.emptyText(false))
		case row.stack != nil:
			lines[i] = renderStackHeader(row.stack, selected, layout)
		default:
			lines[i] = renderPR(v.ordered[row.prIndex], row.indent, selected, layout)
		}
	}

	window := m.windowHeight()
	start := 0
	if len(lines) > window {
		// Keep the selection roughly centred once the list scrolls.
		start = min(max(0, selectedLine-window/2), len(lines)-window)
		// Do not open on an orphaned heading's content: if the window starts
		// right after a repo heading or a stack's header, pull it in.
		if start > 0 && (v.rows[start-1].heading != "" || v.rows[start-1].stack != nil) {
			start--
		}
		// And never open on the spacer above a heading.
		if start < len(v.rows) && v.rows[start].spacer {
			start++
		}
	}
	end := min(len(lines), start+window)
	return strings.Join(lines[start:end], "\n")
}

func indent(s string, n int) string {
	pad := strings.Repeat(" ", n)
	return pad + strings.ReplaceAll(s, "\n", "\n"+pad)
}

// newModel wires up the initial state. repos is the tracked list; on a first
// run (no config file yet) the app opens on the repo picker. The current
// directory's repo is pre-tracked by resolveRepos on a first run, so it shows
// under "current directory" rather than "saved".
func newModel(repos []string, currentRepo string, firstRun bool) model {
	sp := spinner.New()
	// A horizontal dot that bounces left to right, so the loader lies flat
	// on the line instead of the tall rotating braille block.
	sp.Spinner = spinner.Spinner{
		Frames: []string{"●∙∙", "∙●∙", "∙∙●", "∙●∙"},
		FPS:    time.Second / 5,
	}
	// Dim grey so the spinner reads as a quiet placeholder, not a highlight.
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	ti := textinput.New()
	ti.Placeholder = "owner/repo"
	ti.CharLimit = 200
	ti.Width = 40

	saved := slices.Clone(repos)
	if firstRun {
		saved = nil
	}

	m := model{
		width:       80,
		repos:       repos,
		savedRepos:  saved,
		currentRepo: currentRepo,
		input:       ti,
		spinner:     sp,
		firstRun:    firstRun,
	}
	// Mark what Init will start loading so the first frame already shows
	// the spinners (see Init).
	if m.firstRun {
		m.mode = modeConfig
		m.suggestLoading = true
	} else if len(m.repos) > 0 {
		m.cur().loading = true
	}
	return m
}

// Run starts the interactive list. repos is the tracked set, currentRepo the
// repo of the current directory (offered on the config screen), firstRun
// opens on the repo picker and saves what is chosen when the user leaves it,
// and experimental is the opt-in features from the config file.
func Run(repos []string, currentRepo string, firstRun bool, experimental config.Experimental) error {
	m := newModel(repos, currentRepo, firstRun)
	m.experimental = experimental

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running program: %w", err)
	}
	return nil
}
