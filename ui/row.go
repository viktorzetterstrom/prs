package ui

import (
	"fmt"
	"slices"
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
	ghDone      = lipgloss.AdaptiveColor{Light: "#8250df", Dark: "#ab7df8"} // fgColor-done (merged)
)

// statusLabel is the word shown after a PR's title for its github Status.
type statusLabel struct {
	text  string
	style lipgloss.Style
}

func label(text string, color lipgloss.TerminalColor) statusLabel {
	return statusLabel{text, lipgloss.NewStyle().Foreground(color)}
}

// statusLabels covers the statuses a PR in either tab can have. "open" means
// nothing is flagged and no review is required, so it has no label. Merged
// is purple and closed red, as on GitHub. Failures and
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
	"merged":            label("merged", ghDone),
	"closed":            label("closed", ghDanger),
}

var (
	// stackTagStyle is for a layer's position in its stack, such as 2.
	stackTagStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	// treeStyle draws the line that joins a stack's header to its layers.
	treeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

// treeWidth is the cells a tree glyph and the space after it take at the
// start of a stack's rows.
const treeWidth = 2

// prLead is the part of a row before the title, styled, and its width in
// cells: the position in a stack if the PR is in one, size, number, and the
// space after each. The order matches the copied Slack line.
func prLead(pr github.PR) (lead string, width int) {
	if tag := pr.StackTag(); tag != "" {
		lead = stackTagStyle.Render(tag) + " "
		width = ansi.StringWidth(tag) + 1
	}
	stats := fmt.Sprintf("(+%d/-%d)", pr.Additions, pr.Deletions)
	number := fmt.Sprintf("[#%d]", pr.Number)
	lead += statsStyle.Render(stats) + " " + numberStyle.Render(number) + " "
	width += ansi.StringWidth(stats) + 1 + ansi.StringWidth(number) + 1
	return lead, width
}

// rowEntry is one PR on its own, or the layers of one stack, bottom first.
type rowEntry struct {
	pr    github.PR
	stack []github.PR
}

// groupStacks lays prs out for the list. Each PR stays where it is, except
// that the layers of a stack are gathered where the first of them appears,
// bottom layer first: the order they are reviewed and merged in. PRs carry
// a stack only when stack support is on, so otherwise nothing moves.
func groupStacks(prs []github.PR) []rowEntry {
	type key struct {
		repo   string
		number int
	}
	var entries []rowEntry
	at := map[key]int{}
	for _, pr := range prs {
		if pr.Stack == nil {
			entries = append(entries, rowEntry{pr: pr})
			continue
		}
		k := key{pr.Repo, pr.Stack.Number}
		i, ok := at[k]
		if !ok {
			i = len(entries)
			at[k] = i
			entries = append(entries, rowEntry{})
		}
		entries[i].stack = append(entries[i].stack, pr)
	}
	for _, e := range entries {
		slices.SortFunc(e.stack, func(a, b github.PR) int { return a.Stack.Position - b.Stack.Position })
	}
	return entries
}

// stackHeading is the size and text of a stack's header row: the layers'
// sizes added up, and how many PRs the stack has. They match the first line
// of github.FormatStackForSlack.
func stackHeading(layers []github.PR) (stats, text string) {
	add, del := github.StackSize(layers)
	return fmt.Sprintf("(+%d/-%d)", add, del), github.StackTitle(layers)
}

// blocksStack is the statuses that keep a layer, and so its stack, from
// merging.
var blocksStack = map[string]bool{
	"draft":             true,
	"conflicting":       true,
	"ci_failing":        true,
	"changes_requested": true,
	"review_required":   true,
}

// stackLabel is the label on a stack's header. Merging a layer merges every
// layer below it, so it names the lowest layer that holds the stack back,
// as in "#156 needs review"; when none does, it is the top layer's label.
func stackLabel(layers []github.PR) (statusLabel, bool) {
	for _, pr := range layers {
		if s := pr.Status(); blocksStack[s] {
			l := statusLabels[s]
			return statusLabel{fmt.Sprintf("#%d %s", pr.Number, l.text), l.style}, true
		}
	}
	l, ok := statusLabels[layers[len(layers)-1].Status()]
	return l, ok
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
// zero (unknown) never truncates. Stack headers, and the tree glyphs in front
// of stack layers, count towards the longest row.
func newRowLayout(prs []github.PR, width int) rowLayout {
	longest, widestLabel := 0, 0
	fit := func(rowWidth int, l statusLabel, hasLabel bool) {
		longest = max(longest, rowWidth)
		if hasLabel {
			widestLabel = max(widestLabel, ansi.StringWidth(l.text))
		}
	}
	for _, e := range groupStacks(prs) {
		if e.stack == nil {
			_, lead := prLead(e.pr)
			l, ok := statusLabels[e.pr.Status()]
			fit(lead+ansi.StringWidth(e.pr.Title), l, ok)
			continue
		}
		stats, text := stackHeading(e.stack)
		l, ok := stackLabel(e.stack)
		fit(ansi.StringWidth(stats)+1+ansi.StringWidth(text), l, ok)
		for _, pr := range e.stack {
			_, lead := prLead(pr)
			l, ok := statusLabels[pr.Status()]
			fit(treeWidth+lead+ansi.StringWidth(pr.Title), l, ok)
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

// fitTitle cuts title to end within budget cells, when the layout truncates.
func fitTitle(title string, budget int, layout rowLayout) string {
	if layout.truncate && budget > 0 && ansi.StringWidth(title) > budget {
		// Cut one cell short and drop trailing spaces so the ellipsis
		// follows a letter: "Conflicts…", not "Conflicts …".
		return strings.TrimRight(ansi.Truncate(title, budget-1, ""), " ") + "…"
	}
	return title
}

// labelAfter pads a row that is width cells wide out to the label column
// and adds the label.
func labelAfter(width int, l statusLabel, layout rowLayout) string {
	pad := max(labelGap, layout.labelCol-width)
	return strings.Repeat(" ", pad) + l.style.Render(l.text)
}

// selectable adds the row prefix: the selection arrow, or the padding that
// stands in for it.
func selectable(str string, selected bool) string {
	if selected {
		return selectedItemStyle.Render("➤ " + str)
	}
	return itemStyle.Render(str)
}

// renderPR draws one PR line: size, number, title, then its status label in
// the shared column. The title is truncated to end before that column. A
// layer of a stack has tree in front, the glyph that joins it to the
// stack's header.
func renderPR(pr github.PR, tree string, selected bool, layout rowLayout) string {
	leadText, lead := prLead(pr)
	if tree != "" {
		leadText = treeStyle.Render(tree) + " " + leadText
		lead += treeWidth
	}
	title := fitTitle(pr.Title, layout.labelCol-labelGap-lead, layout)
	str := leadText + title
	if l, ok := statusLabels[pr.Status()]; ok {
		str += labelAfter(lead+ansi.StringWidth(title), l, layout)
	}
	return selectable(str, selected)
}

// renderStackHeader draws the row above a stack's layers: their combined
// size, how many PRs the stack has, and the stack's label in the shared
// column. Like the first line of the copied stack, it has no tree glyph.
func renderStackHeader(layers []github.PR, selected bool, layout rowLayout) string {
	stats, text := stackHeading(layers)
	lead := ansi.StringWidth(stats) + 1
	text = fitTitle(text, layout.labelCol-labelGap-lead, layout)
	str := statsStyle.Render(stats) + " " + text
	if l, ok := stackLabel(layers); ok {
		str += labelAfter(lead+ansi.StringWidth(text), l, layout)
	}
	return selectable(str, selected)
}
