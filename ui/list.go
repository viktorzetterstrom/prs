package ui

import (
	"fmt"
	"io"
	"prs/github"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("255"))
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	numberStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	statsStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	copiedStyle       = lipgloss.NewStyle().PaddingLeft(4).Foreground(lipgloss.Color("10")).Bold(true)
)

type item struct {
	pr github.PR
}

func (i item) FilterValue() string { return i.pr.Title }

type itemDelegate struct {
	copied bool
}

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
	str := fmt.Sprintf("%s %s %s", stats, pr.Title, number)

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return selectedItemStyle.Render("➤ " + strings.Join(s, " "))
		}
	}

	fmt.Fprint(w, fn(str))
}

type model struct {
	list   list.Model
	prs    []github.PR
	copied bool
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "q", "ctrl+c":
			return m, tea.Quit

		case " ", "enter":
			if len(m.prs) > 0 && m.list.Index() < len(m.prs) {
				selectedPR := m.prs[m.list.Index()]
				formatted := selectedPR.FormatForSlack()
				err := clipboard.WriteAll(formatted)
				if err == nil {
					m.copied = true
					return m, tea.Tick(time.Millisecond*1500, func(t time.Time) tea.Msg {
						return resetCopiedMsg{}
					})
				}
			}
			return m, nil

		default:
			m.copied = false
		}

	case resetCopiedMsg:
		m.copied = false
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

type resetCopiedMsg struct{}

func (m model) View() string {
	if m.copied {
		copiedMsg := copiedStyle.Render("✓ Copied to clipboard!")
		return m.list.View() + "\n\n" + copiedMsg
	}
	return m.list.View()
}

func Run(prs []github.PR) error {
	items := make([]list.Item, len(prs))
	for i, pr := range prs {
		items[i] = item{pr: pr}
	}

	const defaultWidth = 80

	listHeight := len(prs) + 1
	maxHeight := 16
	if listHeight > maxHeight {
		listHeight = maxHeight
	}

	l := list.New(items, itemDelegate{}, defaultWidth, listHeight)
	l.Title = ""
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false) // Disable pagination for clean display
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle

	m := model{list: l, prs: prs}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running program: %w", err)
	}

	return nil
}
