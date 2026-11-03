package ui

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
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
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	numberStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	statsStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	repoStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	copiedStyle       = lipgloss.NewStyle().PaddingLeft(4).Foreground(lipgloss.Color("10")).Bold(true)
	timestampStyle    = lipgloss.NewStyle().PaddingLeft(4).Foreground(lipgloss.Color("241"))
	errorStyle        = lipgloss.NewStyle().PaddingLeft(4).Foreground(lipgloss.Color("9")).Bold(true)
	hintStyle         = lipgloss.NewStyle().PaddingLeft(4).Foreground(lipgloss.Color("241"))
	activeTabStyle    = lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1).Foreground(lipgloss.Color("255")).Background(lipgloss.Color("33")).Bold(true)
	inactiveTabStyle  = lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1).Foreground(lipgloss.Color("241"))
	tabBarStyle       = lipgloss.NewStyle().PaddingLeft(2).PaddingBottom(1)
	checkedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	uncheckedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

type item struct {
	pr       github.PR
	showRepo bool
}

func (i item) FilterValue() string { return i.pr.Title }

type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	pr := i.pr
	statsRaw := fmt.Sprintf("(+%d/-%d)", pr.Additions, pr.Deletions)
	numberRaw := fmt.Sprintf("[#%d]", pr.Number)
	statusEmoji := pr.StatusEmoji()

	// The repo tag is only shown when more than one repo is tracked, so the
	// single-repo layout is unchanged.
	repoRaw := ""
	if i.showRepo {
		repoRaw = pr.ShortRepo()
	}

	// Budget the title so the right-hand status emoji is always visible. The
	// selected and unselected rows both consume 4 cells before content (4
	// spaces of padding, or 2 spaces + "➤ "), and one single-space separator
	// sits between each pair of content segments. Leave 1 trailing cell as
	// breathing room — some terminals half-clip a 2-cell emoji sitting at the
	// very last column.
	title := pr.Title
	if listWidth := m.Width(); listWidth > 0 {
		separators := 3
		fixed := 4 + ansi.StringWidth(statsRaw) + ansi.StringWidth(numberRaw) + ansi.StringWidth(statusEmoji) + 1
		if repoRaw != "" {
			separators++
			fixed += ansi.StringWidth(repoRaw)
		}
		fixed += separators
		if budget := listWidth - fixed; budget > 0 && ansi.StringWidth(title) > budget {
			title = ansi.Truncate(title, budget, "…")
		}
	}

	segments := []string{statsStyle.Render(statsRaw)}
	if repoRaw != "" {
		segments = append(segments, repoStyle.Render(repoRaw))
	}
	segments = append(segments, title, numberStyle.Render(numberRaw), statusEmoji)
	str := strings.Join(segments, " ")

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return selectedItemStyle.Render("➤ " + strings.Join(s, " "))
		}
	}

	fmt.Fprint(w, fn(str))
}

type tickMsg time.Time

type refreshMsg struct {
	kind github.QueryKind
	gen  int
	prs  []github.PR
	errs []github.RepoError
}

type resetCopiedMsg struct{}

type openedMsg struct{ err error }

type viewState struct {
	kind        github.QueryKind
	prs         []github.PR
	loaded      bool
	loading     bool
	err         error
	lastUpdated time.Time
}

func (v *viewState) reset() {
	v.prs = nil
	v.loaded = false
	v.loading = false
	v.err = nil
	v.lastUpdated = time.Time{}
}

type mode int

const (
	modeList mode = iota
	modeConfig
	modeAddRepo
)

type model struct {
	list    list.Model
	views   []*viewState
	active  int
	copied  bool
	openErr error

	// repos is the effective list of tracked repositories. It starts from the
	// config file, falling back to the current directory's repo when the
	// config is empty, and is written back to the config file whenever the
	// user changes it on the config screen.
	repos       []string
	currentRepo string
	// gen is bumped whenever repos change so in-flight fetches for the old
	// repo set are discarded when they land.
	gen int

	mode      mode
	cfgCursor int
	cfgDirty  bool
	cfgErr    error
	input     textinput.Model
}

func (m model) currentView() *viewState { return m.views[m.active] }

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

func fetchPRs(kind github.QueryKind, repos []string, gen int) tea.Cmd {
	repos = slices.Clone(repos)
	return func() tea.Msg {
		prs, errs := github.GetPRs(kind, repos)
		return refreshMsg{kind: kind, gen: gen, prs: prs, errs: errs}
	}
}

func (m *model) startFetch(v *viewState) tea.Cmd {
	if len(m.repos) == 0 {
		return nil
	}
	v.loading = true
	return fetchPRs(v.kind, m.repos, m.gen)
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) }),
		m.startFetch(m.currentView()),
	)
}

func formatTimeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < 5*time.Second:
		return "Updated just now"
	case d < time.Minute:
		return fmt.Sprintf("Updated %ds ago", int(d.Seconds()))
	default:
		return fmt.Sprintf("Updated %dm ago", int(d.Minutes()))
	}
}

func (m *model) syncListToActive() {
	v := m.currentView()
	items := make([]list.Item, len(v.prs))
	for i, pr := range v.prs {
		items[i] = item{pr: pr, showRepo: m.multiRepo()}
	}
	m.list.SetItems(items)
	if len(v.prs) == 0 {
		m.list.Select(0)
	} else if m.list.Index() >= len(v.prs) {
		m.list.Select(len(v.prs) - 1)
	}
}

func (m *model) ensureLoaded() tea.Cmd {
	v := m.currentView()
	if v.loaded || v.loading {
		return nil
	}
	return m.startFetch(v)
}

// applyRepoChange persists the repo list and invalidates every view so the
// next visit refetches against the new set.
func (m *model) applyRepoChange() {
	m.cfgDirty = true
	m.gen++
	m.cfgErr = config.Config{Repos: m.repos}.Save()
	for _, v := range m.views {
		v.reset()
	}
	if m.cfgCursor >= m.configRowCount() {
		m.cfgCursor = max(0, m.configRowCount()-1)
	}
}

// configRows lists the repos shown on the config screen: every tracked repo,
// plus the current directory's repo as an untracked candidate if it is not
// already in the list.
func (m model) configRows() []string {
	rows := slices.Clone(m.repos)
	if m.currentRepo != "" && !slices.Contains(rows, m.currentRepo) {
		rows = append(rows, m.currentRepo)
	}
	return rows
}

func (m model) configRowCount() int { return len(m.configRows()) }

func (m *model) leaveConfig() tea.Cmd {
	m.mode = modeList
	if !m.cfgDirty {
		return nil
	}
	m.cfgDirty = false
	m.syncListToActive()
	return m.ensureLoaded()
}

func (m model) updateConfig(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := m.configRows()
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc", "c":
		return m, m.leaveConfig()
	case "up", "k":
		if m.cfgCursor > 0 {
			m.cfgCursor--
		}
	case "down", "j":
		if m.cfgCursor < len(rows)-1 {
			m.cfgCursor++
		}
	case " ", "enter":
		if m.cfgCursor < len(rows) {
			repo := rows[m.cfgCursor]
			if idx := slices.Index(m.repos, repo); idx >= 0 {
				m.repos = slices.Delete(m.repos, idx, idx+1)
			} else {
				m.repos = append(m.repos, repo)
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
			m.cfgErr = fmt.Errorf("expected OWNER/REPO, got %q", repo)
			return m, nil
		}
		m.mode = modeConfig
		m.input.Blur()
		if !slices.Contains(m.repos, repo) {
			m.repos = append(m.repos, repo)
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
		m.list.SetWidth(msg.Width)
		return m, nil

	case tickMsg:
		cmds := []tea.Cmd{tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })}
		v := m.currentView()
		if m.mode == modeList && v.loaded && !v.loading && time.Since(v.lastUpdated) >= refreshInterval {
			cmds = append(cmds, m.startFetch(v))
		}
		return m, tea.Batch(cmds...)

	case refreshMsg:
		if msg.gen != m.gen {
			// Result for a repo set the user has since changed; drop it.
			return m, nil
		}
		for _, v := range m.views {
			if v.kind != msg.kind {
				continue
			}
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
			if v == m.currentView() {
				m.syncListToActive()
			}
			break
		}
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeConfig:
			return m.updateConfig(msg)
		case modeAddRepo:
			return m.updateAddRepo(msg)
		}

		switch keypress := msg.String(); keypress {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "tab", "shift+tab":
			if keypress == "tab" {
				m.active = (m.active + 1) % len(m.views)
			} else {
				m.active = (m.active - 1 + len(m.views)) % len(m.views)
			}
			m.copied = false
			m.openErr = nil
			m.syncListToActive()
			return m, m.ensureLoaded()

		case "c":
			m.mode = modeConfig
			m.copied = false
			m.openErr = nil
			m.cfgErr = nil
			m.cfgCursor = 0
			return m, nil

		case "r":
			v := m.currentView()
			if !v.loading {
				return m, m.startFetch(v)
			}
			return m, nil

		case " ", "enter":
			v := m.currentView()
			if len(v.prs) > 0 && m.list.Index() < len(v.prs) {
				selectedPR := v.prs[m.list.Index()]
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
			v := m.currentView()
			if len(v.prs) > 0 && m.list.Index() < len(v.prs) {
				m.openErr = nil
				return m, openInBrowser(v.prs[m.list.Index()].URL)
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

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) renderTabs() string {
	parts := make([]string, len(m.views))
	for i, v := range m.views {
		label := v.kind.Label()
		if i == m.active {
			parts[i] = activeTabStyle.Render(label)
		} else {
			parts[i] = inactiveTabStyle.Render(label)
		}
	}
	return tabBarStyle.Render(strings.Join(parts, " "))
}

func (m model) viewConfig() string {
	var b strings.Builder
	b.WriteString(tabBarStyle.Render(activeTabStyle.Render("Tracked repos")))
	b.WriteString("\n")

	rows := m.configRows()
	if len(rows) == 0 {
		b.WriteString(itemStyle.Render("No repos yet. Press a to add one."))
		b.WriteString("\n")
	}
	for i, repo := range rows {
		tracked := slices.Contains(m.repos, repo)
		mark := uncheckedStyle.Render("○")
		if tracked {
			mark = checkedStyle.Render("✓")
		}
		label := repo
		if repo == m.currentRepo {
			label += repoStyle.Render("  (current directory)")
		}
		line := mark + " " + label
		if i == m.cfgCursor && m.mode == modeConfig {
			b.WriteString(selectedItemStyle.Render("➤ " + line))
		} else {
			b.WriteString(itemStyle.Render(line))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if m.mode == modeAddRepo {
		b.WriteString(itemStyle.Render("Add repo (OWNER/REPO): " + m.input.View()))
		b.WriteString("\n")
		if m.cfgErr != nil {
			b.WriteString(errorStyle.Render(fmt.Sprintf("⚠ %v", m.cfgErr)))
			b.WriteString("\n")
		}
		b.WriteString(hintStyle.Render("enter add · esc cancel"))
	} else {
		if m.cfgErr != nil {
			b.WriteString(errorStyle.Render(fmt.Sprintf("⚠ Could not save config: %v", m.cfgErr)))
			b.WriteString("\n")
		}
		b.WriteString(hintStyle.Render("space toggle · a add · esc back · q quit"))
	}
	return b.String()
}

func (m model) View() string {
	if m.mode != modeList {
		return m.viewConfig()
	}

	v := m.currentView()

	var body string
	switch {
	case len(m.repos) == 0:
		body = itemStyle.Render("No repos tracked. Press c to add one.")
	case v.loading && !v.loaded:
		body = itemStyle.Render("Loading…")
	case v.err != nil && !v.loaded:
		body = errorStyle.Render(fmt.Sprintf("⚠ Could not load PRs:\n%v", indent(v.err.Error(), 2)))
	case len(v.prs) == 0:
		body = itemStyle.Render("No pull requests found.")
	default:
		body = m.list.View()
	}

	var footer strings.Builder
	if m.copied {
		footer.WriteString("\n" + copiedStyle.Render("✓ Copied to clipboard!"))
	}
	if m.openErr != nil {
		footer.WriteString("\n" + errorStyle.Render(fmt.Sprintf("⚠ Could not open browser: %v", m.openErr)))
	}
	if v.err != nil && v.loaded {
		footer.WriteString("\n" + errorStyle.Render(fmt.Sprintf("⚠ Refresh failed:\n%v", indent(v.err.Error(), 2))))
	}
	switch {
	case v.loading && v.loaded:
		footer.WriteString("\n" + timestampStyle.Render("Refreshing…"))
	case v.loaded:
		footer.WriteString("\n" + timestampStyle.Render(formatTimeAgo(v.lastUpdated)))
	}

	return m.renderTabs() + "\n" + body + footer.String()
}

func indent(s string, n int) string {
	pad := strings.Repeat(" ", n)
	return pad + strings.ReplaceAll(s, "\n", "\n"+pad)
}

// resolveRepos builds the initial tracked-repo list: the config file wins,
// otherwise fall back to the repo of the current directory (today's behaviour
// for users without a config).
func resolveRepos(currentRepo string) ([]string, error) {
	if github.DemoMode {
		return slices.Clone(github.DemoRepos), nil
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("could not read config: %w", err)
	}
	if len(cfg.Repos) > 0 {
		return cfg.Repos, nil
	}
	if currentRepo != "" {
		return []string{currentRepo}, nil
	}
	return nil, nil
}

func Run() error {
	const defaultWidth = 80
	const listHeight = 16

	l := list.New(nil, itemDelegate{}, defaultWidth, listHeight)
	l.Title = ""
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle

	views := []*viewState{
		{kind: github.QueryActive},
		{kind: github.QueryLastWeek},
	}

	currentRepo := ""
	if !github.DemoMode {
		currentRepo = github.CurrentRepo()
	}
	repos, err := resolveRepos(currentRepo)
	if err != nil {
		return err
	}

	ti := textinput.New()
	ti.Placeholder = "owner/repo"
	ti.CharLimit = 200
	ti.Width = 40

	m := model{
		list:        l,
		views:       views,
		active:      0,
		repos:       repos,
		currentRepo: currentRepo,
		input:       ti,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running program: %w", err)
	}
	return nil
}
