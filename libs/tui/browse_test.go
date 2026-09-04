package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/oceane-vlt/todolist/libs/ui"

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
	return newBrowseModel(newTestItems(n), "test")
}

// newTestItems builds n items, every other one carrying a description.
func newTestItems(n int) []*todo.Item {
	items := make([]*todo.Item, n)
	for i := range items {
		items[i] = &todo.Item{Title: "item"}
		if i%2 == 0 {
			items[i].Description = "a description long enough to wrap onto several lines when folded open"
		}
	}
	return items
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

// newEditableTestModel wires a browser to a recording editor, so tests can
// assert what would have been sent to the server.
func newEditableTestModel(n int, err error) (model, *[]editCall) {
	calls := &[]editCall{}
	m := newBrowseModel(newTestItems(n), "test")
	m.editor = func(index int, title, description *string) error {
		*calls = append(*calls, editCall{index: index, title: title, description: description})
		return err
	}
	m.adder = func(title, description string) error {
		*calls = append(*calls, editCall{index: -1, title: &title, description: &description})
		return err
	}
	return m, calls
}

type editCall struct {
	index       int
	title       *string
	description *string
}

// drain runs a command and feeds its message back, the way the runtime would.
func drain(t *testing.T, m model, cmd tea.Cmd) model {
	t.Helper()
	if cmd == nil {
		return m
	}
	return sendBrowse(t, m, cmd())
}

// TestEditOpensAndReturnsToTheList is the core of the flow: saving an edit must
// land the user back in the list, not drop them out of the browser.
func TestEditOpensAndReturnsToTheList(t *testing.T) {
	m, calls := newEditableTestModel(3, nil)
	m.cursor = 1

	m = sendBrowse(t, m, browseKey("e"))
	if m.mode != modeEdit {
		t.Fatal("e should open the form")
	}
	if m.editing != 1 {
		t.Errorf("editing item %d, want the one under the cursor (1)", m.editing)
	}

	// Change the title, then save.
	m = sendBrowse(t, m, typeRunes("!"))
	next, cmd := m.Update(browseKey("ctrl+s"))
	m = next.(model)

	if m.mode != modeList {
		t.Error("saving should return to the list, not close the browser")
	}
	m = drain(t, m, cmd)

	if len(*calls) != 1 {
		t.Fatalf("editor called %d times, want once", len(*calls))
	}
	if (*calls)[0].index != 1 {
		t.Errorf("editor got index %d, want 1", (*calls)[0].index)
	}
	// Only the title changed, so only the title travels.
	if (*calls)[0].title == nil {
		t.Error("the changed title should be sent")
	}
	if (*calls)[0].description != nil {
		t.Error("an untouched description must not be sent")
	}
	if m.items[1].Title != *(*calls)[0].title {
		t.Errorf("the row still shows %q, want the saved title", m.items[1].Title)
	}
}

// TestEditCancelReturnsWithoutSaving covers the other exit from the form.
func TestEditCancelReturnsWithoutSaving(t *testing.T) {
	m, calls := newEditableTestModel(3, nil)
	before := m.items[0].Title

	m = sendBrowse(t, m, browseKey("e"))
	m = sendBrowse(t, m, typeRunes("zzz"))
	m = sendBrowse(t, m, browseKey("esc"))

	if m.mode != modeList {
		t.Error("esc should return to the list")
	}
	if len(*calls) != 0 {
		t.Error("cancelling must not call the editor")
	}
	if m.items[0].Title != before {
		t.Errorf("title became %q, want it untouched", m.items[0].Title)
	}
}

// TestEditWithNoChangeSkipsTheRoundTrip: an unchanged form should not pretend to
// save, nor talk to the server.
func TestEditWithNoChangeSkipsTheRoundTrip(t *testing.T) {
	m, calls := newEditableTestModel(3, nil)

	m = sendBrowse(t, m, browseKey("e"))
	next, cmd := m.Update(browseKey("ctrl+s"))
	m = next.(model)

	if cmd != nil {
		t.Error("an unchanged edit should produce no command")
	}
	if len(*calls) != 0 {
		t.Error("an unchanged edit must not call the editor")
	}
	if m.message != "" {
		t.Errorf("message = %q, want none for a no-op edit", m.message)
	}
}

// TestEditFailureIsSurfacedAndNotApplied: a failed save must not leave the list
// showing text the server never accepted.
func TestEditFailureIsSurfacedAndNotApplied(t *testing.T) {
	m, _ := newEditableTestModel(3, errors.New("connection refused"))
	before := m.items[0].Title

	m = sendBrowse(t, m, browseKey("e"))
	m = sendBrowse(t, m, typeRunes("!"))
	next, cmd := m.Update(browseKey("ctrl+s"))
	m = drain(t, next.(model), cmd)

	if m.items[0].Title != before {
		t.Errorf("a failed save changed the row to %q", m.items[0].Title)
	}
	if !strings.Contains(m.message, "connection refused") {
		t.Errorf("message = %q, want the reason surfaced", m.message)
	}
}

// TestEditPreservesMarksAndCursor is why the form is embedded rather than run as
// its own program: an edit must not cost the user their ticks or their place.
func TestEditPreservesMarksAndCursor(t *testing.T) {
	m, _ := newEditableTestModel(6, nil)
	m.cursor = 4
	m = sendBrowse(t, m, browseKey("x")) // tick item 4
	m.cursor = 2
	m = sendBrowse(t, m, browseKey("x")) // tick item 2

	m = sendBrowse(t, m, browseKey("e"))
	m = sendBrowse(t, m, typeRunes("!"))
	next, cmd := m.Update(browseKey("ctrl+s"))
	m = drain(t, next.(model), cmd)

	if m.cursor != 2 {
		t.Errorf("cursor moved to %d, want it left at 2", m.cursor)
	}
	got := m.markedIndices()
	if len(got) != 2 || got[0] != 2 || got[1] != 4 {
		t.Errorf("marks = %v, want [2 4] preserved across the edit", got)
	}
}

// TestEditKeyIsInertWithoutAnEditor keeps a read-only browser honest: the key
// says so instead of doing nothing.
func TestEditKeyIsInertWithoutAnEditor(t *testing.T) {
	m := sendBrowse(t, newTestModel(3), browseKey("e"))
	if m.mode != modeList {
		t.Error("e must not open the form when no editor was supplied")
	}
	if m.message == "" {
		t.Error("e should explain that editing is unavailable")
	}
}

// TestClearingADescriptionFoldsTheRow: an unfolded row whose description was
// just emptied cannot stay unfolded.
func TestClearingADescriptionFoldsTheRow(t *testing.T) {
	m, _ := newEditableTestModel(3, nil)
	m.expanded[0] = true

	m = m.handleEditApplied(editAppliedMsg{index: 0, title: "kept", description: ""})
	if m.expanded[0] {
		t.Error("a row with no description left must not stay unfolded")
	}
}

// TestMessageStyleMatchesItsNature guards against reporting a success in the
// error colour, which reads as a failure.
func TestMessageStyleMatchesItsNature(t *testing.T) {
	m, _ := newEditableTestModel(3, nil)

	saved := m.handleEditApplied(editAppliedMsg{index: 0, title: "kept", description: "kept"})
	if saved.messageIsError {
		t.Error("a successful save must not be flagged as an error")
	}
	if !strings.Contains(saved.footer(), ui.BoldGreen) {
		t.Error("a successful save should use the success style")
	}

	failed := m.handleEditApplied(editAppliedMsg{index: 0, err: errors.New("boom")})
	if !failed.messageIsError {
		t.Error("a failed save must be flagged as an error")
	}
	if !strings.Contains(failed.footer(), ui.BoldRed) {
		t.Error("a failed save should use the error style")
	}
}

// TestAddOpensAnEmptyForm: "a" must not inherit the focused item's text.
func TestAddOpensAnEmptyForm(t *testing.T) {
	m, _ := newEditableTestModel(3, nil)
	m.cursor = 1

	m = sendBrowse(t, m, browseKey("a"))
	if m.mode != modeAdd {
		t.Fatal("a should open the form in add mode")
	}
	title, description := m.edit.values()
	if title != "" || description != "" {
		t.Errorf("the new-item form starts with %q / %q, want both empty", title, description)
	}
	if !strings.Contains(m.View(), headingAdd) {
		t.Errorf("the form should be headed %q so an add is not mistaken for an edit", headingAdd)
	}
}

// TestAddAppendsAndFocusesTheNewItem: after saving, the row must exist and the
// cursor must be on it, so the user sees what they just created.
func TestAddAppendsAndFocusesTheNewItem(t *testing.T) {
	m, calls := newEditableTestModel(3, nil)
	before := len(m.items)

	m = sendBrowse(t, m, browseKey("a"))
	m = sendBrowse(t, m, typeRunes("new task"))
	next, cmd := m.Update(browseKey("ctrl+s"))
	m = drain(t, next.(model), cmd)

	if len(*calls) != 1 || (*calls)[0].index != -1 {
		t.Fatalf("expected exactly one add call, got %+v", *calls)
	}
	if len(m.items) != before+1 {
		t.Fatalf("list holds %d items, want %d", len(m.items), before+1)
	}
	if got := m.items[len(m.items)-1].Title; got != "new task" {
		t.Errorf("appended item title = %q, want %q", got, "new task")
	}
	if m.cursor != len(m.items)-1 {
		t.Errorf("cursor at %d, want it on the new last item %d", m.cursor, len(m.items)-1)
	}
	if m.messageIsError {
		t.Error("a successful add must not be reported as an error")
	}
}

// TestAddRequiresATitle: an empty form cannot be saved, and must stay open.
func TestAddRequiresATitle(t *testing.T) {
	m, calls := newEditableTestModel(3, nil)

	m = sendBrowse(t, m, browseKey("a"))
	next, cmd := m.Update(browseKey("ctrl+s"))
	m = next.(model)

	if m.mode != modeAdd {
		t.Error("a refused save must leave the form open")
	}
	if cmd != nil || len(*calls) != 0 {
		t.Error("a refused save must not reach the adder")
	}
}

// TestAddFailureIsSurfacedAndNotApplied: a rejected add must not leave a row the
// server never accepted.
func TestAddFailureIsSurfacedAndNotApplied(t *testing.T) {
	m, _ := newEditableTestModel(3, errors.New("connection refused"))
	before := len(m.items)

	m = sendBrowse(t, m, browseKey("a"))
	m = sendBrowse(t, m, typeRunes("ghost"))
	next, cmd := m.Update(browseKey("ctrl+s"))
	m = drain(t, next.(model), cmd)

	if len(m.items) != before {
		t.Errorf("a failed add left %d items, want %d", len(m.items), before)
	}
	if !strings.Contains(m.message, "connection refused") {
		t.Errorf("message = %q, want the reason surfaced", m.message)
	}
	if !m.messageIsError {
		t.Error("a failed add should be styled as an error")
	}
}

// TestAddCancelChangesNothing covers the escape route.
func TestAddCancelChangesNothing(t *testing.T) {
	m, calls := newEditableTestModel(3, nil)
	before := len(m.items)

	m = sendBrowse(t, m, browseKey("a"))
	m = sendBrowse(t, m, typeRunes("discarded"))
	m = sendBrowse(t, m, browseKey("esc"))

	if m.mode != modeList {
		t.Error("esc should return to the list")
	}
	if len(*calls) != 0 || len(m.items) != before {
		t.Error("cancelling an add must change nothing")
	}
}

// TestAddKeyIsInertWithoutAnAdder mirrors the editor case.
func TestAddKeyIsInertWithoutAnAdder(t *testing.T) {
	m := sendBrowse(t, newTestModel(3), browseKey("a"))
	if m.mode != modeList {
		t.Error("a must not open the form when no adder was supplied")
	}
	if m.message == "" || !m.messageIsError {
		t.Error("a should explain that adding is unavailable")
	}
}

// TestHintFallsBackByWidth pins the tiering: the widest hint that fits wins, and
// the alternate keys are what gets given up first.
func TestHintFallsBackByWidth(t *testing.T) {
	widest := browseHints[0]
	if got := hintFor(runeLen(widest)); got != widest {
		t.Errorf("at exactly its own width, hintFor returned %q", got)
	}
	if !strings.Contains(widest, "x/space") || !strings.Contains(widest, "→/enter") {
		t.Error("the widest hint should spell out the alternate keys, as users asked")
	}

	// One cell too narrow: step down, never wrap.
	if got := hintFor(runeLen(widest) - 1); got == widest {
		t.Error("hintFor should step down when the widest hint does not fit")
	}

	// Every tier must be strictly shorter than the one before it, or the ladder
	// would have a rung that never gets used.
	for i := 1; i < len(browseHints); i++ {
		if runeLen(browseHints[i]) >= runeLen(browseHints[i-1]) {
			t.Errorf("hint %d is not shorter than hint %d", i, i-1)
		}
	}

	// Absurdly narrow: still the shortest tier, and the footer clamps it.
	if got := hintFor(1); got != browseHints[len(browseHints)-1] {
		t.Errorf("hintFor(1) = %q, want the shortest tier", got)
	}
}
