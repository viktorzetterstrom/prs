package ui

import (
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/viktorzetterstrom/prs/config"
	"github.com/viktorzetterstrom/prs/github"
)

type suggestMsg struct {
	repos []string
	err   error
}

func fetchSuggestions() tea.Msg {
	repos, err := github.SuggestRepos()
	return suggestMsg{repos: repos, err: err}
}

func (m *model) ensureSuggestions() tea.Cmd {
	if m.suggestFetched || m.suggestLoading {
		return nil
	}
	m.suggestLoading = true
	return tea.Batch(fetchSuggestions, m.spinner.Tick)
}

func (m *model) saveConfig() {
	if github.DemoMode {
		// Demo mode must never touch the real config file; the in-memory
		// repo list still changes so the UI can be exercised.
		return
	}
	m.cfgErr = config.Config{Repos: m.repos, Experimental: m.experimental}.Save()
}

// experiment is one row of the config screen's experimental section: an
// opt-in feature that may change or go away.
type experiment struct {
	name  string
	about string
	// flag points at the setting's field in a config.Experimental.
	flag func(*config.Experimental) *bool
}

var experiments = []experiment{
	{
		name:  "stack support",
		about: "show each pr's place in its github stack",
		flag:  func(e *config.Experimental) *bool { return &e.Stacks },
	},
}

// applyChange persists the config and invalidates the view so the next
// visit refetches with the new repos and settings.
func (m *model) applyChange() {
	m.cfgDirty = true
	m.gen++
	m.saveConfig()
	for i := range m.tabs {
		m.tabs[i].reset()
	}
	if m.cfgCursor >= m.configRowCount() {
		m.cfgCursor = max(0, m.configRowCount()-1)
	}
}

const suggestedTitle = "suggested from your recent prs"

// aboutStyle is the dim explanation after an experimental setting's name.
var aboutStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

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
	add(suggestedTitle, m.suggestions)
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

// configRowCount is every row the cursor can reach: the repos, then the
// experimental settings below them.
func (m model) configRowCount() int { return len(m.configRows()) + len(experiments) }

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

func (m model) updateConfig(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := m.configRows()
	key := msg.String()
	if cursor, ok := moveCursor(key, m.cfgCursor, m.configRowCount(), 5); ok {
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
		switch i := m.cfgCursor - len(rows); {
		case i < 0:
			repo := rows[m.cfgCursor]
			if idx := slices.Index(m.repos, repo); idx >= 0 {
				m.repos = slices.Delete(m.repos, idx, idx+1)
			} else {
				m.trackRepo(repo)
			}
			m.applyChange()
		case i < len(experiments):
			flag := experiments[i].flag(&m.experimental)
			*flag = !*flag
			m.applyChange()
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
			m.applyChange()
		}
		m.cfgCursor = slices.Index(m.configRows(), repo)
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
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

	// While suggestions load, show their section heading with a bare spinner
	// beneath, like the startup skeleton, so the section is visible from the
	// start and fills in place. Afterwards the section renders through
	// sections() above; only the empty and error cases need a line here.
	hasSuggestedSection := false
	for _, sec := range secs {
		if sec.title == suggestedTitle {
			hasSuggestedSection = true
		}
	}
	switch {
	case m.suggestLoading && !hasSuggestedSection:
		if len(secs) > 0 {
			b.WriteString("\n")
		}
		b.WriteString(sectionStyle.Render(suggestedTitle))
		b.WriteString("\n")
		b.WriteString(itemStyle.Render(strings.TrimRight(m.spinner.View(), " ")))
		b.WriteString("\n")
	case m.suggestErr != nil:
		b.WriteString("\n" + errorStyle.Render(fmt.Sprintf("⚠ could not load suggestions: %v", m.suggestErr)))
		b.WriteString("\n")
	case m.suggestFetched && len(m.suggestions) == 0:
		b.WriteString("\n" + timestampStyle.Render("no recent pr activity found to suggest from."))
		b.WriteString("\n")
	}

	// The experimental settings come after every repo row, so their cursor
	// rows are numbered on from the repos'.
	b.WriteString("\n" + sectionStyle.Render("experimental") + "\n")
	for i, e := range experiments {
		mark := uncheckedStyle.Render("○")
		if *e.flag(&m.experimental) {
			mark = checkedStyle.Render("✓")
		}
		line := mark + " " + e.name + "  " + aboutStyle.Render(e.about)
		if row+i == m.cfgCursor && m.mode == modeConfig {
			b.WriteString(selectedItemStyle.Render("➤ " + line))
		} else {
			b.WriteString(itemStyle.Render(line))
		}
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
