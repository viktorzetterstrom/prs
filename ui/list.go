package ui

import (
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	copiedStyle       = lipgloss.NewStyle().PaddingLeft(4).Foreground(lipgloss.Color("10")).Bold(true)
	timestampStyle    = lipgloss.NewStyle().PaddingLeft(4).Foreground(lipgloss.Color("241"))
	errorStyle        = lipgloss.NewStyle().PaddingLeft(4).Foreground(lipgloss.Color("9")).Bold(true)
	activeTabStyle    = lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1).Foreground(lipgloss.Color("255")).Background(lipgloss.Color("33")).Bold(true)
	inactiveTabStyle  = lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1).Foreground(lipgloss.Color("241"))
	tabBarStyle       = lipgloss.NewStyle().PaddingLeft(2).PaddingBottom(1)
)

type item struct {
	pr github.PR
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
	stats := statsStyle.Render(fmt.Sprintf("(+%d/-%d)", pr.Additions, pr.Deletions))
	number := numberStyle.Render(fmt.Sprintf("[#%d]", pr.Number))
	statusEmoji := pr.StatusEmoji()
	str := fmt.Sprintf("%s %s %s %s", stats, pr.Title, number, statusEmoji)

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
	prs  []github.PR
	err  error
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

type model struct {
	list    list.Model
	views   []*viewState
	active  int
	copied  bool
	openErr error
}

func (m model) currentView() *viewState { return m.views[m.active] }

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

func fetchPRs(kind github.QueryKind) tea.Cmd {
	return func() tea.Msg {
		prs, err := github.GetPRs(kind)
		return refreshMsg{kind: kind, prs: prs, err: err}
	}
}

func (m model) Init() tea.Cmd {
	v := m.currentView()
	v.loading = true
	return tea.Batch(
		tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) }),
		fetchPRs(v.kind),
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
		items[i] = item{pr: pr}
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
	v.loading = true
	return fetchPRs(v.kind)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case tickMsg:
		cmds := []tea.Cmd{tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })}
		v := m.currentView()
		if v.loaded && !v.loading && time.Since(v.lastUpdated) >= refreshInterval {
			v.loading = true
			cmds = append(cmds, fetchPRs(v.kind))
		}
		return m, tea.Batch(cmds...)

	case refreshMsg:
		for _, v := range m.views {
			if v.kind != msg.kind {
				continue
			}
			v.loading = false
			v.lastUpdated = time.Now()
			if msg.err == nil {
				v.prs = msg.prs
				v.loaded = true
				v.err = nil
			} else {
				v.err = msg.err
			}
			if v == m.currentView() {
				m.syncListToActive()
			}
			break
		}
		return m, nil

	case tea.KeyMsg:
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

		case "r":
			v := m.currentView()
			if !v.loading {
				v.loading = true
				return m, fetchPRs(v.kind)
			}
			return m, nil

		case " ", "enter":
			v := m.currentView()
			if len(v.prs) > 0 && m.list.Index() < len(v.prs) {
				selectedPR := v.prs[m.list.Index()]
				formatted := selectedPR.FormatForSlack()
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

func (m model) View() string {
	v := m.currentView()

	var body string
	switch {
	case v.loading && !v.loaded:
		body = itemStyle.Render("Loading…")
	case v.err != nil && !v.loaded:
		body = errorStyle.Render(fmt.Sprintf("⚠ Could not load PRs: %v", v.err))
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
		footer.WriteString("\n" + errorStyle.Render(fmt.Sprintf("⚠ Refresh failed: %v", v.err)))
	}
	switch {
	case v.loading && v.loaded:
		footer.WriteString("\n" + timestampStyle.Render("Refreshing…"))
	case v.loaded:
		footer.WriteString("\n" + timestampStyle.Render(formatTimeAgo(v.lastUpdated)))
	}

	return m.renderTabs() + "\n" + body + footer.String()
}

func Run(initial github.QueryKind) error {
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
		{kind: github.QueryActionable},
		{kind: github.QueryLastWeek},
	}
	active := 0
	for i, v := range views {
		if v.kind == initial {
			active = i
			break
		}
	}

	m := model{list: l, views: views, active: active}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running program: %w", err)
	}
	return nil
}
