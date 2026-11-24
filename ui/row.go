package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/viktorzetterstrom/prs/github"
)

// GitHub's own colours, so a status reads here the way it does on
// github.com: the foreground tokens of its Primer design system
// (@primer/primitives 11.10.0), with the light and dark theme values.
var (
	ghSuccess   = lipgloss.AdaptiveColor{Light: "#1a7f37", Dark: "#3fb950"} // fgColor-success, -open
	ghDanger    = lipgloss.AdaptiveColor{Light: "#d1242f", Dark: "#f85149"} // fgColor-danger, -closed
	ghAttention = lipgloss.AdaptiveColor{Light: "#9a6700", Dark: "#d29922"} // fgColor-attention
	ghNeutral   = lipgloss.AdaptiveColor{Light: "#59636e", Dark: "#9198a1"} // fgColor-neutral, -draft, -muted
)

// statusLabel is the word shown after a PR's title for its github Status.
type statusLabel struct {
	text  string
	style lipgloss.Style
}

func label(text string, color lipgloss.TerminalColor) statusLabel {
	return statusLabel{text, lipgloss.NewStyle().Foreground(color)}
}

// statusLabels covers the statuses an open PR can have. "open" means nothing
// is flagged and no review is required, so it has no label. Failures and
// requested changes are red as on GitHub, approval green, and drafts grey.
// Waiting on reviewers is grey too, as review state is in GitHub's PR list.
// GitHub documents no colour for the merge queue; it gets attention yellow,
// GitHub's colour for something under way, like running checks.
var statusLabels = map[string]statusLabel{
	"draft":             label("draft", ghNeutral),
	"queued":            label("queued", ghAttention),
	"conflicting":       label("conflicts", ghDanger),
	"ci_failing":        label("ci failing", ghDanger),
	"changes_requested": label("changes requested", ghDanger),
	"approved":          label("approved", ghSuccess),
	"review_required":   label("needs review", ghNeutral),
}

// prLead is the part of a row before the title: size, number and the spaces
// after each. The order matches the copied Slack line.
func prLead(pr github.PR) (stats, number string, width int) {
	stats = fmt.Sprintf("(+%d/-%d)", pr.Additions, pr.Deletions)
	number = fmt.Sprintf("[#%d]", pr.Number)
	return stats, number, ansi.StringWidth(stats) + 1 + ansi.StringWidth(number) + 1
}

// rowLayout is shared by every PR row so the status labels line up in one
// column. labelCol is where that column starts, counted from the first cell
// after the row's 4-cell prefix (4 spaces of padding, or 2 spaces + "➤ ").
// Titles are cut to end before it only when truncate is set.
type rowLayout struct {
	labelCol int
	truncate bool
}

// labelGap is the space between the end of the longest title and the labels.
const labelGap = 2

// newRowLayout puts the label column just after the longest row, so it sits
// close to the titles on a wide terminal. When that does not fit in width,
// the column moves left to leave room for the widest label and titles are
// truncated to meet it. 1 trailing cell is left as breathing room. A width of
// zero (unknown) never truncates.
func newRowLayout(prs []github.PR, width int) rowLayout {
	longest, widestLabel := 0, 0
	for _, pr := range prs {
		_, _, lead := prLead(pr)
		longest = max(longest, lead+ansi.StringWidth(pr.Title))
		if l, ok := statusLabels[pr.Status()]; ok {
			widestLabel = max(widestLabel, ansi.StringWidth(l.text))
		}
	}
	if width <= 0 {
		return rowLayout{labelCol: longest + labelGap}
	}
	reserve := 0
	if widestLabel > 0 {
		reserve = labelGap + widestLabel
	}
	return rowLayout{labelCol: min(longest, width-4-1-reserve) + labelGap, truncate: true}
}

// renderPR draws one PR line: size, number, title, then its status label in
// the shared column. The title is truncated to end before that column.
func renderPR(pr github.PR, selected bool, layout rowLayout) string {
	stats, number, lead := prLead(pr)
	titleEnd := layout.labelCol - labelGap

	title := pr.Title
	if budget := titleEnd - lead; layout.truncate && budget > 0 && ansi.StringWidth(title) > budget {
		// Cut one cell short and drop trailing spaces so the ellipsis
		// follows a letter: "Conflicts…", not "Conflicts …".
		title = strings.TrimRight(ansi.Truncate(title, budget-1, ""), " ") + "…"
	}

	str := statsStyle.Render(stats) + " " + numberStyle.Render(number) + " " + title
	if l, ok := statusLabels[pr.Status()]; ok {
		pad := max(labelGap, layout.labelCol-lead-ansi.StringWidth(title))
		str += strings.Repeat(" ", pad) + l.style.Render(l.text)
	}
	if selected {
		return selectedItemStyle.Render("➤ " + str)
	}
	return itemStyle.Render(str)
}
