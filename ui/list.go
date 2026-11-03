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
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/viktorzetterstrom/prs/config"
	"github.com/viktorzetterstrom/prs/github"
)

const refreshInterval = 30 * time.Second

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
	sectionStyle      = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("245")).Bold(true)
	headerStyle       = lipgloss.NewStyle().PaddingLeft(2).PaddingBottom(1)
	checkedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	uncheckedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

// listRow is one line of the PR list: a repo heading, a PR, or an "empty
// repo" note. Only PR rows are selectable; prIndex indexes model.ordered.
type listRow struct {
	heading string
	prIndex int
	empty   bool
	spacer  bool
}

// renderPR draws one PR line. The title is truncated so the trailing PR
// number is always visible: both the selected and unselected rows consume 4
// cells before content (4 spaces of padding, or 2 spaces + "➤ "), two
// single-space separators sit between the three segments, and 1 trailing
// cell is left as breathing room.
func renderPR(pr github.PR, selected bool, width int) string {
	statsRaw := fmt.Sprintf("(+%d/-%d)", pr.Additions, pr.Deletions)
	numberRaw := fmt.Sprintf("[#%d]", pr.Number)

	title := pr.Title
	if width > 0 {
		fixed := 4 + ansi.StringWidth(statsRaw) + ansi.StringWidth(numberRaw) + 2 + 1
		if budget := width - fixed; budget > 0 && ansi.StringWidth(title) > budget {
			title = ansi.Truncate(title, budget, "…")
		}
	}

	str := statsStyle.Render(statsRaw) + " " + title + " " + numberStyle.Render(numberRaw)
	if selected {
		return selectedItemStyle.Render("➤ " + str)
	}
	return itemStyle.Render(str)
}

type tickMsg time.Time

type refreshMsg struct {
	gen  int
	prs  []github.PR
	errs []github.RepoError
}

type suggestMsg struct {
	repos []string
	err   error
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

type mode int

const (
	modeList mode = iota
	modeConfig
	modeAddRepo
)

type model struct {
	view    viewState
	copied  bool
	openErr error

	// ordered is view.prs regrouped by tracked repo (in m.repos order), and
	// rows is the rendered structure over it. cursor indexes ordered.
	ordered []github.PR
	rows    []listRow
	cursor  int
	width   int
	height  int

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

	mode      mode
	cfgCursor int
	cfgDirty  bool
	cfgErr    error
	input     textinput.Model
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

func fetchPRs(repos []string, gen int) tea.Cmd {
	repos = slices.Clone(repos)
	return func() tea.Msg {
		prs, errs := github.GetPRs(repos)
		return refreshMsg{gen: gen, prs: prs, errs: errs}
	}
}

func fetchSuggestions() tea.Msg {
	repos, err := github.SuggestRepos()
	return suggestMsg{repos: repos, err: err}
}

func (m *model) startFetch() tea.Cmd {
	if len(m.repos) == 0 {
		return nil
	}
	m.view.loading = true
	return fetchPRs(m.repos, m.gen)
}

func (m *model) ensureSuggestions() tea.Cmd {
	if m.suggestFetched || m.suggestLoading {
		return nil
	}
	m.suggestLoading = true
	return fetchSuggestions
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) }),
	}
	if m.mode == modeConfig {
		cmds = append(cmds, m.ensureSuggestions())
	} else {
		cmds = append(cmds, m.startFetch())
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

// syncList regroups the fetched PRs by repo. With several repos tracked each
// repo gets a heading (in tracked order) and repos with nothing open say so;
// with one repo the list is flat. The cursor is clamped to the new length.
func (m *model) syncList() {
	m.ordered = m.ordered[:0]
	m.rows = m.rows[:0]

	if !m.multiRepo() {
		for _, pr := range m.view.prs {
			m.rows = append(m.rows, listRow{prIndex: len(m.ordered)})
			m.ordered = append(m.ordered, pr)
		}
	} else {
		for i, repo := range m.repos {
			if i > 0 {
				m.rows = append(m.rows, listRow{spacer: true})
			}
			m.rows = append(m.rows, listRow{heading: github.PR{Repo: repo}.ShortRepo()})
			found := false
			for _, pr := range m.view.prs {
				if pr.Repo != repo {
					continue
				}
				found = true
				m.rows = append(m.rows, listRow{prIndex: len(m.ordered)})
				m.ordered = append(m.ordered, pr)
			}
			if !found {
				m.rows = append(m.rows, listRow{empty: true})
			}
		}
	}

	if m.cursor >= len(m.ordered) {
		m.cursor = max(0, len(m.ordered)-1)
	}
}

func (m model) selectedPR() (github.PR, bool) {
	if m.cursor < 0 || m.cursor >= len(m.ordered) {
		return github.PR{}, false
	}
	return m.ordered[m.cursor], true
}

func (m *model) ensureLoaded() tea.Cmd {
	if m.view.loaded || m.view.loading {
		return nil
	}
	return m.startFetch()
}

func (m *model) saveConfig() {
	if github.DemoMode {
		// Demo mode must never touch the real config file; the in-memory
		// repo list still changes so the UI can be exercised.
		return
	}
	m.cfgErr = config.Config{Repos: m.repos}.Save()
}

// applyRepoChange persists the repo list and invalidates the view so the
// next visit refetches against the new set.
func (m *model) applyRepoChange() {
	m.cfgDirty = true
	m.gen++
	m.saveConfig()
	m.view.reset()
	if m.cfgCursor >= m.configRowCount() {
		m.cfgCursor = max(0, m.configRowCount()-1)
	}
}

// repoSection is one group of rows on the config screen.
type repoSection struct {
	title string
	repos []string
}

// sections groups the picker rows by where they came from. Order inside each
// section is stable: saved repos keep their config order (additions by name
// append), and suggestions keep the order the search returned them in. A repo
// appears only in the first section that lists it. Toggling a repo changes its
// checkmark, not its row, so the cursor stays on what the user is looking at.
func (m model) sections() []repoSection {
	var secs []repoSection
	seen := map[string]bool{}
	add := func(title string, repos []string) {
		var rows []string
		for _, r := range repos {
			if r != "" && !seen[r] {
				seen[r] = true
				rows = append(rows, r)
			}
		}
		if len(rows) > 0 {
			secs = append(secs, repoSection{title: title, repos: rows})
		}
	}
	add("saved", m.savedRepos)
	add("current directory", []string{m.currentRepo})
	add("suggested from your recent prs", m.suggestions)
	return secs
}

// configRows flattens sections into the cursor's row order.
func (m model) configRows() []string {
	var rows []string
	for _, sec := range m.sections() {
		rows = append(rows, sec.repos...)
	}
	return rows
}

// trackRepo adds repo to the tracked set, keeping m.repos in display order so
// the config file reads top to bottom like the picker does.
func (m *model) trackRepo(repo string) {
	rows := m.configRows()
	if !slices.Contains(rows, repo) {
		m.savedRepos = append(m.savedRepos, repo)
		rows = m.configRows()
	}
	var repos []string
	for _, r := range rows {
		if r == repo || slices.Contains(m.repos, r) {
			repos = append(repos, r)
		}
	}
	m.repos = repos
}

func (m model) configRowCount() int { return len(m.configRows()) }

func (m *model) leaveConfig() tea.Cmd {
	m.mode = modeList
	if m.firstRun && len(m.repos) > 0 {
		if !m.cfgDirty {
			// User accepted the preselected repos: persist them so the picker
			// does not greet them again next time.
			m.saveConfig()
		}
		// The first visit ends once the user leaves with something selected.
		m.firstRun = false
	}
	if !m.cfgDirty {
		return m.ensureLoaded()
	}
	m.cfgDirty = false
	m.syncList()
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

func (m model) updateConfig(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := m.configRows()
	key := msg.String()
	if cursor, ok := moveCursor(key, m.cfgCursor, len(rows), 5); ok {
		m.cfgCursor = cursor
		return m, nil
	}
	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc", "enter", "c":
		// Checklist convention: space toggles, enter confirms.
		return m, m.leaveConfig()
	case " ":
		if m.cfgCursor < len(rows) {
			repo := rows[m.cfgCursor]
			if idx := slices.Index(m.repos, repo); idx >= 0 {
				m.repos = slices.Delete(m.repos, idx, idx+1)
			} else {
				m.trackRepo(repo)
			}
			m.applyRepoChange()
		}
	case "a":
		m.mode = modeAddRepo
		m.cfgErr = nil
		m.input.SetValue("")
		return m, m.input.Focus()
	}
	return m, nil
}

func (m model) updateAddRepo(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode = modeConfig
		m.input.Blur()
		return m, nil
	case "enter":
		repo := strings.TrimSpace(m.input.Value())
		if !github.ValidRepo(repo) {
			m.cfgErr = fmt.Errorf("expected owner/repo, got %q", repo)
			return m, nil
		}
		m.mode = modeConfig
		m.input.Blur()
		if !slices.Contains(m.repos, repo) {
			m.trackRepo(repo)
			m.applyRepoChange()
		}
		m.cfgCursor = slices.Index(m.configRows(), repo)
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		cmds := []tea.Cmd{tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })}
		v := &m.view
		if m.mode == modeList && v.loaded && !v.loading && time.Since(v.lastUpdated) >= refreshInterval {
			cmds = append(cmds, m.startFetch())
		}
		return m, tea.Batch(cmds...)

	case refreshMsg:
		if msg.gen != m.gen {
			// Result for a repo set the user has since changed; drop it.
			return m, nil
		}
		v := &m.view
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
		m.suggestLoading = false
		m.suggestFetched = true
		m.suggestErr = msg.err
		m.suggestions = msg.repos
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeConfig:
			return m.updateConfig(msg)
		case modeAddRepo:
			return m.updateAddRepo(msg)
		}

		key := msg.String()
		if cursor, ok := moveCursor(key, m.cursor, len(m.ordered), m.pageSize()); ok {
			m.cursor = cursor
			m.copied = false
			m.openErr = nil
			return m, nil
		}

		switch key {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "c":
			m.mode = modeConfig
			m.copied = false
			m.openErr = nil
			m.cfgErr = nil
			m.cfgCursor = 0
			return m, m.ensureSuggestions()

		case "r":
			if !m.view.loading {
				return m, m.startFetch()
			}
			return m, nil

		case "enter", "y":
			// Enter is the primary action; y is vim's "yank" for copy.
			if selectedPR, ok := m.selectedPR(); ok {
				formatted := selectedPR.FormatForSlack(m.multiRepo())
				if err := clipboard.WriteAll(formatted); err == nil {
					m.copied = true
					return m, tea.Tick(time.Millisecond*1500, func(t time.Time) tea.Msg {
						return resetCopiedMsg{}
					})
				}
			}
			return m, nil

		case "o":
			if pr, ok := m.selectedPR(); ok {
				m.openErr = nil
				return m, openInBrowser(pr.URL)
			}
			return m, nil

		default:
			m.copied = false
			m.openErr = nil
		}

	case resetCopiedMsg:
		m.copied = false
		return m, nil

	case openedMsg:
		m.openErr = msg.err
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

func (m model) renderHeader() string {
	return headerStyle.Render(titleStyle.Render("open prs"))
}

func (m model) viewConfig() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render(titleStyle.Render("config")))
	b.WriteString("\n")

	if github.DemoMode {
		b.WriteString(timestampStyle.Render("demo mode: changes here are not saved."))
		b.WriteString("\n\n")
	}

	secs := m.sections()
	if len(secs) == 0 && !m.suggestLoading {
		b.WriteString(itemStyle.Render("no repos yet. press a to add one."))
		b.WriteString("\n")
	}
	row := 0
	for si, sec := range secs {
		if si > 0 {
			b.WriteString("\n")
		}
		b.WriteString(sectionStyle.Render(sec.title))
		b.WriteString("\n")
		for _, repo := range sec.repos {
			mark := uncheckedStyle.Render("○")
			if slices.Contains(m.repos, repo) {
				mark = checkedStyle.Render("✓")
			}
			line := mark + " " + repo
			if row == m.cfgCursor && m.mode == modeConfig {
				b.WriteString(selectedItemStyle.Render("➤ " + line))
			} else {
				b.WriteString(itemStyle.Render(line))
			}
			b.WriteString("\n")
			row++
		}
	}

	switch {
	case m.suggestLoading:
		b.WriteString("\n" + timestampStyle.Render("looking up repos you've opened prs in…"))
		b.WriteString("\n")
	case m.suggestErr != nil:
		b.WriteString("\n" + errorStyle.Render(fmt.Sprintf("⚠ could not load suggestions: %v", m.suggestErr)))
		b.WriteString("\n")
	case m.suggestFetched && len(m.suggestions) == 0:
		b.WriteString("\n" + timestampStyle.Render("no recent pr activity found to suggest from."))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if m.mode == modeAddRepo {
		b.WriteString(itemStyle.Render("add repo (owner/repo): " + m.input.View()))
		b.WriteString("\n")
		if m.cfgErr != nil {
			b.WriteString(errorStyle.Render(fmt.Sprintf("⚠ %v", m.cfgErr)))
			b.WriteString("\n")
		}
		b.WriteString(hints("enter", "add", "esc", "cancel"))
	} else {
		if m.cfgErr != nil {
			b.WriteString(errorStyle.Render(fmt.Sprintf("⚠ could not save config: %v", m.cfgErr)))
			b.WriteString("\n")
		}
		b.WriteString(hints("↑/↓", "move", "space", "toggle", "a", "add by name", "enter", "done"))
	}
	return b.String()
}

func (m model) View() string {
	if m.mode != modeList {
		return m.viewConfig()
	}

	v := m.view

	var body string
	switch {
	case len(m.repos) == 0:
		body = itemStyle.Render("no repos tracked yet. press c to pick some.")
	case v.loading && !v.loaded:
		body = itemStyle.Render("loading…")
	case v.err != nil && !v.loaded:
		body = errorStyle.Render(fmt.Sprintf("⚠ could not load prs:\n%v", indent(v.err.Error(), 2)))
	case len(v.prs) == 0:
		body = itemStyle.Render("no open pull requests.")
	default:
		body = m.renderRows()
	}

	// The footer always occupies the same number of lines so transient
	// messages (copied, browser error) never push the timestamp or the key
	// legend around. Persistent refresh errors are the one exception: they
	// are multi-line and stay until the next successful fetch.
	var footer strings.Builder
	if v.err != nil && v.loaded {
		footer.WriteString("\n" + errorStyle.Render(fmt.Sprintf("⚠ refresh failed:\n%v", indent(v.err.Error(), 2))))
	}
	status := ""
	switch {
	case m.copied:
		status = copiedStyle.Render("✓ copied to clipboard!")
	case m.openErr != nil:
		status = errorStyle.Render(fmt.Sprintf("⚠ could not open browser: %v", m.openErr))
	}
	footer.WriteString("\n" + status)
	timestamp := ""
	switch {
	case v.loading && v.loaded:
		timestamp = timestampStyle.Render("refreshing…")
	case v.loaded:
		timestamp = timestampStyle.Render(formatTimeAgo(v.lastUpdated))
	}
	footer.WriteString("\n" + timestamp)
	footer.WriteString("\n\n" + hints("↑/↓", "move", "enter", "copy", "o", "open", "r", "refresh", "c", "config"))

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

// renderRows draws the grouped list, scrolled so the selected PR is visible.
func (m model) renderRows() string {
	lines := make([]string, len(m.rows))
	selectedLine := 0
	for i, row := range m.rows {
		switch {
		case row.spacer:
			lines[i] = ""
		case row.heading != "":
			lines[i] = sectionStyle.Render(row.heading)
		case row.empty:
			lines[i] = emptyStyle.Render("no open pull requests")
		default:
			selected := row.prIndex == m.cursor
			if selected {
				selectedLine = i
			}
			lines[i] = renderPR(m.ordered[row.prIndex], selected, m.width)
		}
	}

	window := m.windowHeight()
	start := 0
	if len(lines) > window {
		// Keep the selection roughly centred once the list scrolls.
		start = min(max(0, selectedLine-window/2), len(lines)-window)
		// Do not open on an orphaned heading's content: if the window starts
		// right after a heading, pull the heading in.
		if start > 0 && m.rows[start-1].heading != "" {
			start--
		}
		// And never open on the spacer above a heading.
		if start < len(m.rows) && m.rows[start].spacer {
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

// resolveRepos builds the initial tracked-repo list: the config file wins,
// otherwise fall back to the repo of the current directory. The bool reports
// whether a config file existed, which decides if the repo picker opens first.
func resolveRepos(currentRepo string) (repos []string, configExists bool, err error) {
	if github.DemoMode {
		return slices.Clone(github.DemoRepos), true, nil
	}
	cfg, found, err := config.Load()
	if err != nil {
		return nil, found, fmt.Errorf("could not read config: %w", err)
	}
	if len(cfg.Repos) > 0 {
		return cfg.Repos, true, nil
	}
	if currentRepo != "" {
		return []string{currentRepo}, found, nil
	}
	return nil, found, nil
}

// newModel wires up the initial state. repos is the tracked list; on a first
// run (no config file yet) the app opens on the repo picker. The current
// directory's repo is pre-tracked by resolveRepos on a first run, so it shows
// under "current directory" rather than "saved".
func newModel(repos []string, currentRepo string, firstRun bool) model {
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
		firstRun:    firstRun,
	}
	if m.firstRun {
		m.mode = modeConfig
	}
	return m
}

func Run() error {
	currentRepo := ""
	if !github.DemoMode {
		currentRepo = github.CurrentRepo()
	}
	repos, configExists, err := resolveRepos(currentRepo)
	if err != nil {
		return err
	}

	m := newModel(repos, currentRepo, !configExists)

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running program: %w", err)
	}
	return nil
}
