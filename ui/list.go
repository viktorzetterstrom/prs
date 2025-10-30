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
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	numberStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	statsStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	copiedStyle       = lipgloss.NewStyle().PaddingLeft(4).Foreground(lipgloss.Color("10")).Bold(true)
)

type item struct {
	pr github.PR
}

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

	l := list.New(items, itemDelegate{}, defaultWidth, 14)
	l.Title = ""
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle

	m := model{list: l, prs: prs}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running program: %w", err)
	}

	return nil
}
