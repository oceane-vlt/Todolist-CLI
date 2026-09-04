package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	todo "github.com/oceane-vlt/todolist/proto"
)

func TestWrap(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		width int
		want  []string
	}{
		{"empty text yields one empty line", "", 20, []string{""}},
		{"blank text yields one empty line", "   \n  ", 20, []string{""}},
		{"short text stays on one line", "leak under the sink", 40, []string{"leak under the sink"}},
		{
			name:  "long text breaks on spaces",
			text:  "leak under the kitchen sink, get a quote first",
			width: 20,
			want:  []string{"leak under the", "kitchen sink, get a", "quote first"},
		},
		{
			// A word longer than the width overflows rather than being cut apart:
			// truncating a URL or an identifier mid-way would make it unusable.
			name:  "oversized word overflows instead of being split",
			text:  "see https://example.com/a/very/long/path now",
			width: 10,
			want:  []string{"see", "https://example.com/a/very/long/path", "now"},
		},
		{
			// Descriptions are multi-line, so the author's breaks are structure,
			// not whitespace to be normalised away.
			name:  "author line breaks are preserved",
			text:  "one\ntwo\nthree",
			width: 40,
			want:  []string{"one", "two", "three"},
		},
		{
			name:  "blank line between paragraphs is kept",
			text:  "first para\n\nsecond para",
			width: 40,
			want:  []string{"first para", "", "second para"},
		},
		{
			name:  "each line wraps independently",
			text:  "a short line\nthis second line is long enough to wrap",
			width: 20,
			want:  []string{"a short line", "this second line is", "long enough to wrap"},
		},
		{
			// Accents are one cell each: counting bytes would wrap this too early.
			name:  "accented text wraps on character count, not bytes",
			text:  "fuite sous l'évier de la cuisine",
			width: 20,
			want:  []string{"fuite sous l'évier", "de la cuisine"},
		},
		{
			name:  "crlf is treated as a single break",
			text:  "one\r\ntwo",
			width: 40,
			want:  []string{"one", "two"},
		},
		{
			name:  "surrounding blank lines are trimmed",
			text:  "\n\n  content  \n\n",
			width: 40,
			want:  []string{"content"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrap(tt.text, tt.width)
			if len(got) != len(tt.want) {
				t.Fatalf("wrap() = %q (%d lines), want %q (%d lines)", got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("line %d = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// newTestModel builds a model over n items, every other one carrying a
// description, which is enough to exercise the marker and folding logic.
func newTestModel(n int) model {
	items := make([]*todo.Item, n)
	for i := range items {
		items[i] = &todo.Item{Title: "item"}
		if i%2 == 0 {
			items[i].Description = "a description long enough to wrap onto several lines when folded open"
		}
	}
	return newBrowseModel(items, "test")
}

func TestHasDescription(t *testing.T) {
	m := newTestModel(3)
	if !m.hasDescription(0) {
		t.Error("item 0 should have a description")
	}
	if m.hasDescription(1) {
		t.Error("item 1 should not have a description")
	}
	// Out-of-range indices must be safe, since the cursor arithmetic feeds them.
	if m.hasDescription(-1) || m.hasDescription(99) {
		t.Error("out-of-range index should report no description")
	}
}

// TestVisibleRangeKeepsCursorOnScreen is the property that matters for a list
// longer than the terminal: whatever the cursor position, it must be rendered.
// Both scroll directions are walked, since the offset only moves as needed.
func TestVisibleRangeKeepsCursorOnScreen(t *testing.T) {
	m := newTestModel(100)

	forward := []int{0, 1, 37, 60, 98, 99}
	backward := []int{99, 98, 60, 37, 1, 0}
	for _, sequence := range [][]int{forward, backward} {
		for _, cursor := range sequence {
			m.cursor = cursor
			m.clampOffset()
			start, end := m.visibleRange()
			if cursor < start || cursor >= end {
				t.Errorf("cursor %d not within visible range [%d,%d)", cursor, start, end)
			}
		}
	}
}

// TestVisibleRangeScrollsMinimally pins the "steady list" behaviour: moving the
// cursor within the visible window must not scroll at all.
func TestVisibleRangeScrollsMinimally(t *testing.T) {
	m := newTestModel(100)
	m.cursor = 0
	m.clampOffset()

	start, end := m.visibleRange()
	if start != 0 {
		t.Fatalf("offset should start at 0, got %d", start)
	}

	// Move to the last item that is already on screen: nothing should scroll.
	m.cursor = end - 1
	m.clampOffset()
	if newStart, _ := m.visibleRange(); newStart != start {
		t.Errorf("moving inside the window scrolled from %d to %d", start, newStart)
	}

	// One step further has to scroll, by the minimum.
	m.cursor = end
	m.clampOffset()
	if newStart, _ := m.visibleRange(); newStart <= start {
		t.Errorf("moving past the window should scroll, offset stayed at %d", newStart)
	}
}

// TestVisibleRangeAccountsForExpandedItems checks that unfolding a description
// costs rows, so fewer items fit on screen.
func TestVisibleRangeAccountsForExpandedItems(t *testing.T) {
	m := newTestModel(100)
	m.cursor = 0
	m.clampOffset()

	startCollapsed, endCollapsed := m.visibleRange()

	m.expanded[0] = true
	m.clampOffset()
	startExpanded, endExpanded := m.visibleRange()

	if (endExpanded - startExpanded) >= (endCollapsed - startCollapsed) {
		t.Errorf("unfolding should reduce the number of visible items, got %d then %d",
			endCollapsed-startCollapsed, endExpanded-startExpanded)
	}
}

// TestVisibleRangeAlwaysShowsAnItem guards the degenerate case of a terminal too
// short for even one row: the view must never come back empty.
func TestVisibleRangeAlwaysShowsAnItem(t *testing.T) {
	m := newTestModel(10)
	m.height = 1
	m.cursor = 4
	m.clampOffset()

	start, end := m.visibleRange()
	if end <= start {
		t.Fatalf("visibleRange() = [%d,%d), want at least one item", start, end)
	}
	if m.cursor < start || m.cursor >= end {
		t.Errorf("cursor %d not visible in [%d,%d)", m.cursor, start, end)
	}
}

// TestViewMarksItemsWithDescription is the user-facing contract: an item with a
// description is visibly distinguished from one without.
func TestViewMarksItemsWithDescription(t *testing.T) {
	m := newTestModel(2)

	out := m.View()
	if !strings.Contains(out, markerCollapsed) {
		t.Errorf("view should mark the item carrying a description, got:\n%s", out)
	}

	m.expanded[0] = true
	out = m.View()
	if !strings.Contains(out, markerExpanded) {
		t.Errorf("unfolded item should use the expanded marker, got:\n%s", out)
	}
	if !strings.Contains(out, "a description long enough") {
		t.Errorf("unfolded item should render its description, got:\n%s", out)
	}
}

// TestViewOmitsDescriptionWhenFolded is the other half: titles only by default.
func TestViewOmitsDescriptionWhenFolded(t *testing.T) {
	m := newTestModel(2)
	if strings.Contains(m.View(), "a description long enough") {
		t.Error("a folded item must not render its description")
	}
}

// TestViewFitsTerminalHeight is the invariant inline rendering depends on: the
// rendered frame must never be taller than the terminal, or Bubble Tea's cursor
// arithmetic corrupts the display. It is checked across terminal sizes, list
// lengths, cursor positions and folded/unfolded states.
func TestViewFitsTerminalHeight(t *testing.T) {
	for _, height := range []int{10, 15, 24, 40, 60} {
		for _, count := range []int{1, 3, 8, 40, 200} {
			m := newTestModel(count)
			m.height = height

			for _, cursor := range []int{0, count / 2, count - 1} {
				m.cursor = cursor
				m.clampOffset()

				// Unfold whatever the cursor sits on, the worst case for height.
				if m.hasDescription(cursor) {
					m.expanded[cursor] = true
					m.clampOffset()
				}

				lines := strings.Count(m.View(), "\n")
				if lines > height {
					t.Errorf("height=%d count=%d cursor=%d: view is %d lines, exceeds the terminal",
						height, count, cursor, lines)
				}
			}
		}
	}
}

// TestViewFitsTinyTerminal covers the degenerate end: even when the terminal is
// shorter than the chrome itself, one item is still shown and the frame stays as
// small as it can.
func TestViewFitsTinyTerminal(t *testing.T) {
	m := newTestModel(20)
	m.height = 3
	m.cursor = 10
	m.clampOffset()

	out := m.View()
	if strings.TrimSpace(out) == "" {
		t.Fatal("view must not be blank on a tiny terminal")
	}
	start, end := m.visibleRange()
	if end-start != 1 {
		t.Errorf("a tiny terminal should show exactly one item, got %d", end-start)
	}
}

// sendBrowse drives the browser model through one message.
func sendBrowse(t *testing.T, m model, msg tea.Msg) model {
	t.Helper()
	next, _ := m.Update(msg)
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned %T, want model", next)
	}
	return got
}

func browseKey(s string) tea.KeyMsg {
	switch s {
	case "ctrl+s":
		return tea.KeyMsg{Type: tea.KeyCtrlS}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func TestMarkTogglesAndReportsInOrder(t *testing.T) {
	m := newTestModel(5)

	// Mark item 2, then 0, out of order.
	m.cursor = 2
	m = sendBrowse(t, m, browseKey("x"))
	m.cursor = 0
	m = sendBrowse(t, m, browseKey("x"))

	// Marks are reported ascending, not in map order.
	if got := m.markedIndices(); len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Errorf("markedIndices() = %v, want [0 2]", got)
	}

	// x again unticks.
	m = sendBrowse(t, m, browseKey("x"))
	if got := m.markedIndices(); len(got) != 1 || got[0] != 2 {
		t.Errorf("after unticking, markedIndices() = %v, want [2]", got)
	}
}

// TestSpaceMarksRatherThanExpands pins the rebinding: space is the checkbox key
// in a list of checkboxes, and unfolding keeps →/enter.
func TestSpaceMarksRatherThanExpands(t *testing.T) {
	m := newTestModel(3)
	m = sendBrowse(t, m, browseKey(" "))

	if !m.marked[0] {
		t.Error("space should mark the focused row")
	}
	if m.expanded[0] {
		t.Error("space should no longer unfold the description")
	}
}

func TestConfirmRequiresAMark(t *testing.T) {
	m := newTestModel(3)

	m = sendBrowse(t, m, browseKey("ctrl+s"))
	if m.confirmed {
		t.Error("confirming with nothing marked should not commit")
	}
	if m.message == "" {
		t.Error("confirming with nothing marked should say so")
	}

	m = sendBrowse(t, m, browseKey("x"))
	m = sendBrowse(t, m, browseKey("ctrl+s"))
	if !m.confirmed {
		t.Error("ctrl+s with a mark should commit")
	}
}

// TestQuitGuardsPendingMarks is the data-loss guard: quitting with ticks must
// not silently throw them away.
func TestQuitGuardsPendingMarks(t *testing.T) {
	m := newTestModel(3)
	m = sendBrowse(t, m, browseKey("x"))

	m = sendBrowse(t, m, browseKey("q"))
	if !m.discardPrompt {
		t.Fatal("quitting with marks should ask for confirmation first")
	}
	if m.confirmed {
		t.Error("the prompt must not commit anything")
	}
	if !strings.Contains(m.footer(), "discard") {
		t.Errorf("the prompt should be visible in the footer, got %q", m.footer())
	}

	// A second q goes through.
	m2 := sendBrowse(t, m, browseKey("q"))
	if m2.discardPrompt {
		t.Error("the second quit should not re-prompt")
	}

	// Whereas any other key cancels the prompt, so it cannot linger.
	m3 := sendBrowse(t, m, browseKey("down"))
	if m3.discardPrompt {
		t.Error("moving after the prompt should cancel it")
	}
}

// TestQuitWithoutMarksDoesNotPrompt keeps the guard from becoming a nuisance on
// the common read-only path.
func TestQuitWithoutMarksDoesNotPrompt(t *testing.T) {
	m := sendBrowse(t, newTestModel(3), browseKey("q"))
	if m.discardPrompt {
		t.Error("quitting with nothing marked should not prompt")
	}
}

// TestCtrlCAlwaysLeaves: an interrupt is not a decision to be confirmed.
func TestCtrlCAlwaysLeaves(t *testing.T) {
	m := newTestModel(3)
	m = sendBrowse(t, m, browseKey("x"))
	m = sendBrowse(t, m, browseKey("ctrl+c"))
	if m.discardPrompt {
		t.Error("ctrl+c should not prompt")
	}
	if m.confirmed {
		t.Error("ctrl+c should not commit marks")
	}
}

func TestClampLine(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		width int
		want  string
	}{
		{"fits untouched", "hello", 10, "hello"},
		{"exact fit untouched", "hello", 5, "hello"},
		{"truncated with ellipsis", "hello world", 8, "hello w…"},
		{"width one", "hello", 1, "…"},
		{"zero width", "hello", 0, ""},
		{"negative width", "hello", -3, ""},
		{"counts characters not bytes", "évier", 5, "évier"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampLine(tt.text, tt.width); got != tt.want {
				t.Errorf("clampLine(%q, %d) = %q, want %q", tt.text, tt.width, got, tt.want)
			}
		})
	}
}

// TestFooterAlwaysFitsOneRow is what lets chromeLines budget a single row for
// the hint: no footer state, at any width, may exceed the terminal.
func TestFooterAlwaysFitsOneRow(t *testing.T) {
	for _, width := range []int{10, 20, 40, 60, 80, 120} {
		m := newTestModel(3)
		m.width = width

		states := map[string]model{"hint": m}

		marked := sendBrowse(t, m, browseKey("x"))
		states["marked"] = marked
		states["discard prompt"] = sendBrowse(t, marked, browseKey("q"))
		states["message"] = sendBrowse(t, m, browseKey("ctrl+s"))

		for name, state := range states {
			// The footer is a leading blank line plus one text row.
			for _, line := range strings.Split(strings.Trim(state.footer(), "\n"), "\n") {
				if got := runeLen(stripANSI(line)); got > width {
					t.Errorf("width=%d state=%s: footer row is %d cells: %q", width, name, got, line)
				}
			}
		}
	}
}

// stripANSI removes escape sequences so a rendered line can be measured in the
// cells it actually occupies.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
