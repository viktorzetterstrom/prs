package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/viktorzetterstrom/prs/github"
)

func TestPRRowMatchesSlackOrder(t *testing.T) {
	pr := github.PR{Number: 42, Title: "Fix the thing", Additions: 1, Deletions: 2, State: "OPEN"}
	got := ansi.Strip(renderPR(pr, false, newRowLayout([]github.PR{pr}, 0)))
	if want := "    (+1/-2) [#42] Fix the thing"; got != want {
		t.Fatalf("row = %q, want %q", got, want)
	}

	pr.Title = strings.Repeat("long title ", 20)
	got = ansi.Strip(renderPR(pr, true, newRowLayout([]github.PR{pr}, 40)))
	if !strings.HasPrefix(got, "  ➤ (+1/-2) [#42] long") || !strings.HasSuffix(got, "…") {
		t.Fatalf("selected row should keep size and number and truncate the title: %q", got)
	}
	if w := ansi.StringWidth(got); w >= 40 {
		t.Fatalf("row is %d cells wide, want under 40: %q", w, got)
	}
}

func TestPRRowsShowAlignedStatusLabels(t *testing.T) {
	prs := []github.PR{
		{Number: 1, Title: "Short", State: "OPEN", ReviewDecision: "APPROVED"},
		{Number: 22, Title: "A much longer title", Additions: 100, State: "OPEN", IsDraft: true},
		{Number: 3, Title: "Nothing flagged", State: "OPEN"},
	}
	layout := newRowLayout(prs, 120)
	rows := make([]string, len(prs))
	for i, pr := range prs {
		rows[i] = ansi.Strip(renderPR(pr, i == 0, layout))
	}

	if !strings.HasSuffix(rows[0], "approved") {
		t.Fatalf("approved row has no label: %q", rows[0])
	}
	if !strings.HasSuffix(rows[1], "A much longer title  draft") {
		t.Fatalf("label should sit two cells after the longest title: %q", rows[1])
	}
	// Compare cells, not bytes: the selection arrow is one cell, three bytes.
	col := func(row, label string) int { return ansi.StringWidth(row[:strings.Index(row, label)]) }
	if col(rows[0], "approved") != col(rows[1], "draft") {
		t.Fatalf("labels are not in one column:\n%s\n%s", rows[0], rows[1])
	}
	if !strings.HasSuffix(rows[2], "Nothing flagged") {
		t.Fatalf("an open PR with nothing flagged should have no label: %q", rows[2])
	}
}

func TestLongTitleIsTruncatedBeforeTheLabel(t *testing.T) {
	pr := github.PR{Number: 7, Title: strings.Repeat("word ", 30), State: "OPEN", ReviewDecision: "CHANGES_REQUESTED"}
	got := ansi.Strip(renderPR(pr, false, newRowLayout([]github.PR{pr}, 60)))
	if !strings.HasSuffix(got, "…  changes requested") {
		t.Fatalf("title should be cut to make room for the label: %q", got)
	}
	if w := ansi.StringWidth(got); w > 59 {
		t.Fatalf("row is %d cells wide, want at most 59: %q", w, got)
	}
}
