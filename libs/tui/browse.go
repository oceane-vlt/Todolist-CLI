// Package tui holds the interactive terminal views of the CLI, built on Bubble
// Tea. It is deliberately separate from libs/ui: libs/ui prints one-shot lines
// to stdout (and stays usable in a pipe or a script), while this package takes
// over the terminal for as long as the user navigates.
//
// Every view here must have a static equivalent in libs/ui, because the CLI
// falls back to it whenever stdout is not a terminal.
package tui

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/oceane-vlt/todolist/libs/ui"
	todo "github.com/oceane-vlt/todolist/proto"
)

// Glyphs used in the fold column. It tells the user, at a glance, which items
// hide a description — the whole point of a view that shows titles only.
const (
	markerCollapsed = "▸"
	markerExpanded  = "▾"
	markerNone      = " "
)

// Glyphs used in the selection gutter. A solid bar is preferred over a pointing
// chevron: it reads as a highlight rather than a glyph, and it renders
// consistently across monospace fonts.
const (
	gutterSelected = "▌"
	gutterBlank    = " "
)

// Checkbox glyphs, kept identical to the static rendering in libs/ui so the
// same list looks the same whichever view prints it.
const (
	checkboxPending   = "[ ]"
	checkboxCompleted = "[✓]"
	// checkboxMarked is a pending item the user has ticked but not yet committed.
	// It is deliberately distinct from checkboxCompleted: nothing has happened to
	// the item yet, and showing it as already done would be a lie.
	checkboxMarked = "[x]"
)

// rowIndent is the left margin of every rendered line.
const rowIndent = "  "

// Column widths of an item row, in terminal cells (every glyph above is one
// cell wide):
//
//	rowIndent │ gutter ␣ │ marker ␣ │ checkbox ␣ │ title
const (
	rowIndentWidth = len(rowIndent)
	gutterWidth    = 1
	markerWidth    = 1
	checkboxWidth  = 3
)

// titleColumn is the cell an item's title starts at. A wrapped description is
// indented to exactly this column, which is what makes it read as belonging to
// the title above it rather than to the row as a whole.
//
// Deriving it from the column widths (instead of hard-coding an indent string)
// is what stops the two from drifting apart when the row layout changes.
const titleColumn = rowIndentWidth + gutterWidth + 1 + markerWidth + 1 + checkboxWidth + 1

// descriptionIndent aligns a wrapped description under its item's title.
var descriptionIndent = strings.Repeat(" ", titleColumn)

// minContentWidth guards the wrapping maths on very narrow (or not yet
// reported) terminals, so a description never wraps to one character per line.
const minContentWidth = 20

// defaultWidth and defaultHeight are used until the terminal reports its real
// size, which Bubble Tea does immediately on start.
const (
	defaultWidth  = 80
	defaultHeight = 24
)

// chromeLines is the number of rows View() spends on anything that is not an
// item, and therefore the amount subtracted from the terminal height to get the
// room left for the list.
//
// It is an exact count, because inline rendering breaks if the view overflows
// the terminal:
//
//	1  leading blank line
//	1  "  <list title>  (n)"
//	1  blank line
//	1  "↑ n more" (only when scrolled down, but always budgeted)
//	1  "↓ n more" (only when more items follow, but always budgeted)
//	2  blank line + the key hints footer
//
// The two "n more" rows are budgeted unconditionally: reserving them costs one
// item of display, while under-counting them would corrupt the rendering on
// exactly the lists that are long enough to need them.
const chromeLines = 7

// model is the browser state: the items on display, which row is focused and
// which descriptions are unfolded.
type model struct {
	listTitle string
	items     []*todo.Item
	cursor    int
	// expanded tracks the unfolded rows by item index. Only items that actually
	// have a description can be unfolded.
	expanded map[int]bool
	// marked tracks the rows ticked for completion, by item index. Nothing is
	// sent to the server until the user commits, so this is purely local intent.
	marked map[int]bool
	// confirmed records that the user committed the marks rather than quitting.
	confirmed bool
	// discardPrompt is set when the user asked to quit while marks were pending,
	// turning the next quit into a confirmation instead of a silent loss.
	discardPrompt bool
	// message is a transient line shown in place of the key hints.
	message string
	width   int
	height  int
	// offset is the index of the first item rendered, moved just enough to keep
	// the cursor on screen.
	offset int
}

// BrowseResult is what the user did in the browser.
//
// Completed holds indices into the slice passed to Browse — not into the
// caller's full list. The browser is given a filtered view (pending items only),
// so the caller owns the mapping back to its own indices; doing it here would
// mean the view had to know what it was filtered from.
type BrowseResult struct {
	Completed []int
	// Confirmed reports that the user committed the marks. It is false when they
	// quit, whatever was ticked.
	Confirmed bool
}

// Browse runs the interactive item browser and blocks until the user quits or
// commits. items should already be filtered to what the caller wants to display.
//
// It returns an error when the terminal program cannot run; the caller is
// expected to fall back to the static rendering in that case rather than leave
// the user with nothing.
func Browse(items []*todo.Item, listTitle string) (BrowseResult, error) {
	m := newBrowseModel(items, listTitle)
	// Deliberately NOT WithAltScreen: the browser renders inline, right where the
	// command was typed, so the surrounding terminal history stays visible and the
	// last frame remains on screen after quitting — the list behaves like command
	// output that happened to be navigable, not like a separate application.
	//
	// Inline rendering makes View()'s height a hard constraint: it must fit in the
	// terminal, or the renderer's cursor arithmetic corrupts the display. That is
	// what chromeLines and rowBudget enforce.
	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return BrowseResult{}, err
	}

	done, ok := final.(model)
	if !ok {
		return BrowseResult{}, fmt.Errorf("tui: unexpected final model %T", final)
	}
	if !done.confirmed {
		return BrowseResult{}, nil
	}
	return BrowseResult{Completed: done.markedIndices(), Confirmed: true}, nil
}

// markedIndices returns the ticked rows in ascending order, so the caller gets a
// deterministic list rather than Go's randomised map order.
func (m model) markedIndices() []int {
	indices := make([]int, 0, len(m.marked))
	for i := range m.marked {
		indices = append(indices, i)
	}
	sort.Ints(indices)
	return indices
}

// newBrowseModel is the ONLY place a browser model is built, tests included: a
// second construction site is how a newly added map ends up nil in one of them.
func newBrowseModel(items []*todo.Item, listTitle string) model {
	return model{
		listTitle: listTitle,
		items:     items,
		expanded:  make(map[int]bool),
		marked:    make(map[int]bool),
		width:     defaultWidth,
		height:    defaultHeight,
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampOffset()
		return m, nil

	case tea.KeyMsg:
		// Ctrl+C is an interrupt, not a decision: it always leaves immediately,
		// discarding marks. Only q/esc get the "are you sure" treatment below.
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// Any keypress clears a transient notice; the two branches that want one
		// set it again below.
		wasPrompting := m.discardPrompt
		m.message = ""
		m.discardPrompt = false

		switch msg.String() {
		case "q", "esc":
			// Quitting with marks pending would silently throw away work the user
			// believes they have done, so the first press explains and the second
			// confirms.
			if len(m.marked) > 0 && !wasPrompting {
				m.discardPrompt = true
				return m, nil
			}
			return m, tea.Quit

		case "x", " ":
			if m.marked[m.cursor] {
				delete(m.marked, m.cursor)
			} else {
				m.marked[m.cursor] = true
			}

		case "ctrl+s":
			if len(m.marked) == 0 {
				m.message = "Nothing marked — press x to tick an item."
				return m, nil
			}
			m.confirmed = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}

		case "home", "g":
			m.cursor = 0

		case "end", "G":
			m.cursor = len(m.items) - 1

		case "right", "l", "enter":
			if m.hasDescription(m.cursor) {
				m.expanded[m.cursor] = true
			}

		case "left", "h":
			delete(m.expanded, m.cursor)

		case "tab":
			// Toggle, for users who expect one key to do both.
			if m.hasDescription(m.cursor) {
				if m.expanded[m.cursor] {
					delete(m.expanded, m.cursor)
				} else {
					m.expanded[m.cursor] = true
				}
			}
		}

		// Every branch above can move the cursor or change how many rows an item
		// takes, so the scroll window is reconciled once, here.
		m.clampOffset()
	}

	return m, nil
}

// hasDescription reports whether the item at i carries a description worth
// unfolding.
func (m model) hasDescription(i int) bool {
	return i >= 0 && i < len(m.items) && strings.TrimSpace(m.items[i].Description) != ""
}

func (m model) View() string {
	var b strings.Builder

	// The title is clamped rather than allowed to wrap: a second physical row
	// would push the frame past the terminal and corrupt the rendering.
	title := clampLine(m.listTitle, m.width-len(rowIndent)-len(itemCount(len(m.items)))-4)
	fmt.Fprintf(&b, "\n%s%s%s%s  %s%s%s",
		rowIndent, ui.Bold, title, ui.ColorReset,
		ui.Dim, itemCount(len(m.items)), ui.ColorReset)
	if n := len(m.marked); n > 0 {
		fmt.Fprintf(&b, "  %s%d marked%s", ui.BoldAccent, n, ui.ColorReset)
	}
	b.WriteString("\n\n")

	if len(m.items) == 0 {
		fmt.Fprintf(&b, "%s%sNothing to show.%s\n", rowIndent, ui.Dim, ui.ColorReset)
		return b.String() + m.footer()
	}

	start, end := m.visibleRange()
	if start > 0 {
		fmt.Fprintf(&b, "%s%s↑ %d more%s\n", rowIndent, ui.Dim, start, ui.ColorReset)
	}
	for i := start; i < end; i++ {
		b.WriteString(m.renderItem(i))
	}
	if end < len(m.items) {
		fmt.Fprintf(&b, "%s%s↓ %d more%s\n", rowIndent, ui.Dim, len(m.items)-end, ui.ColorReset)
	}

	return b.String() + m.footer()
}

// renderItem renders one row plus, when unfolded, its wrapped description.
//
// The styling carries the hierarchy: the title is the content and keeps the
// terminal's default colour; the gutter marks the focused row in the single
// accent colour; everything else (fold arrow, checkbox) is dimmed so it reads
// as structure rather than information.
func (m model) renderItem(i int) string {
	item := m.items[i]

	gutter, gutterStyle := gutterBlank, ""
	titleStyle := ""
	if i == m.cursor {
		gutter, gutterStyle = gutterSelected, ui.BoldAccent
		titleStyle = ui.Bold
	}

	// A folded arrow is structure (dim); an unfolded one is state the user just
	// changed, so it takes the accent to confirm the action.
	marker, markerStyle := markerNone, ""
	switch {
	case !m.hasDescription(i):
	case m.expanded[i]:
		marker, markerStyle = markerExpanded, ui.Accent
	default:
		marker, markerStyle = markerCollapsed, ui.Dim
	}

	// A ticked box is the one piece of chrome that is NOT dimmed: it is pending
	// user intent, the thing they most need to see before committing.
	checkbox, checkboxStyle := checkboxPending, ui.Dim
	if m.marked[i] {
		checkbox, checkboxStyle = checkboxMarked, ui.BoldAccent
	}
	if item.Completed {
		checkbox, checkboxStyle = checkboxCompleted, ui.Dim
		// A completed row recedes entirely, title included.
		titleStyle = ui.Dim
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s%s%s%s %s%s%s %s%s%s %s%s%s\n",
		rowIndent,
		gutterStyle, gutter, ui.ColorReset,
		markerStyle, marker, ui.ColorReset,
		checkboxStyle, checkbox, ui.ColorReset,
		titleStyle, item.Title, ui.ColorReset)

	if m.expanded[i] {
		// Italic, in the default foreground: the indentation already says the text
		// is subordinate, so it does not also need to be faint — a description you
		// opened on purpose has to be comfortable to read.
		for _, line := range wrap(strings.TrimSpace(item.Description), m.contentWidth()) {
			fmt.Fprintf(&b, "%s%s%s%s\n", descriptionIndent, ui.Italic, line, ui.ColorReset)
		}
	}

	return b.String()
}

// Footer texts. They are clamped to the terminal width before being printed,
// because a wrapped hint would occupy two rows where chromeLines budgets one.
const (
	browseHint       = "↑↓ move · → open · x mark · ctrl+s complete · q/esc quit"
	browseHintNarrow = "↑↓ move · x mark · ctrl+s done · q/esc quit"
)

func (m model) footer() string {
	width := m.width - len(rowIndent)

	// A pending discard is the most important thing on screen, so it takes the
	// hint row outright rather than competing with it.
	if m.discardPrompt {
		text := fmt.Sprintf("%d marked but not completed — q again to discard, ctrl+s to complete", len(m.marked))
		return fmt.Sprintf("\n%s%s%s%s\n", rowIndent, ui.BoldRed, clampLine(text, width), ui.ColorReset)
	}
	if m.message != "" {
		return fmt.Sprintf("\n%s%s%s%s\n", rowIndent, ui.BoldRed, clampLine(m.message, width), ui.ColorReset)
	}

	// Prefer the full hint, but drop to the short one before resorting to
	// truncation: a hint cut mid-word teaches the user nothing.
	hint := browseHint
	if runeLen(hint) > width {
		hint = browseHintNarrow
	}
	return fmt.Sprintf("\n%s%s%s%s\n", rowIndent, ui.Dim, clampLine(hint, width), ui.ColorReset)
}

// clampLine truncates s to width cells, marking the cut with an ellipsis. It is
// what guarantees every chrome line occupies exactly one physical row, which is
// the assumption chromeLines encodes.
func clampLine(s string, width int) string {
	if width < 1 {
		return ""
	}
	if runeLen(s) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	return string([]rune(s)[:width-1]) + "…"
}

// itemCount renders the header's count, pluralised like the `todo list` output.
func itemCount(n int) string {
	if n == 1 {
		return "1 item"
	}
	return fmt.Sprintf("%d items", n)
}

// contentWidth is the room left for description text once the indent is taken
// out, floored so wrapping stays sane on a narrow terminal.
func (m model) contentWidth() int {
	w := m.width - titleColumn - len(rowIndent)
	if w < minContentWidth {
		return minContentWidth
	}
	return w
}

// rowBudget is how many terminal rows the item list may occupy.
func (m model) rowBudget() int {
	budget := m.height - chromeLines
	if budget < 1 {
		return 1
	}
	return budget
}

// clampOffset scrolls by the minimum amount needed to keep the cursor on
// screen: up when the cursor moves above the window, down when it falls below.
//
// Scrolling minimally (rather than, say, recentring on the cursor every time)
// is what keeps the list steady under the reader: rows only move when they have
// to, so the eye does not have to re-find its place on every keypress.
func (m *model) clampOffset() {
	if m.offset > m.cursor {
		m.offset = m.cursor
	}
	if m.offset < 0 {
		m.offset = 0
	}

	// Push the window down until the cursor fits within the row budget. Unfolded
	// descriptions are counted in rows, so opening one can scroll the list.
	budget := m.rowBudget()
	for m.offset < m.cursor {
		used := 0
		for i := m.offset; i <= m.cursor; i++ {
			used += m.rowsFor(i)
		}
		if used <= budget {
			break
		}
		m.offset++
	}
}

// visibleRange returns the half-open range of items to render, filling the row
// budget from the current scroll offset. It measures rows rather than items so
// unfolded descriptions are accounted for.
//
// At least one item is always returned, even one taller than the screen, so the
// view is never blank.
func (m model) visibleRange() (start, end int) {
	start = m.offset
	if start >= len(m.items) {
		start = max(0, len(m.items)-1)
	}

	budget := m.rowBudget()
	used := 0
	for end = start; end < len(m.items); end++ {
		rows := m.rowsFor(end)
		if used+rows > budget && end > start {
			break
		}
		used += rows
	}

	return start, end
}

// rowsFor is the number of terminal rows item i occupies: its title line plus
// the wrapped description when unfolded.
func (m model) rowsFor(i int) int {
	rows := 1
	if m.expanded[i] {
		rows += len(wrap(strings.TrimSpace(m.items[i].Description), m.contentWidth()))
	}
	return rows
}

// wrap breaks text into display lines of at most width cells.
//
// The author's own line breaks are preserved — including the blank lines that
// separate paragraphs — and each of those lines is then wrapped on spaces to
// fit. Descriptions may be multi-line, so flattening them into a single
// paragraph would silently destroy the structure the user typed.
//
// A word longer than width overflows its line rather than being cut apart:
// splitting a URL or an identifier in half makes it unusable, whereas an
// overflowing one can still be read and copied.
//
// Width is counted in runes, not bytes, so accented text ("l'évier") wraps at
// the right place. This is still an approximation for double-width glyphs (CJK,
// emoji), which occupy two cells each and will therefore wrap slightly late.
//
// At least one (possibly empty) line is always returned, so callers can range
// over the result unconditionally.
func wrap(text string, width int) []string {
	text = strings.ReplaceAll(strings.TrimSpace(text), "\r\n", "\n")

	lines := []string{}
	for _, paragraph := range strings.Split(text, "\n") {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			// A blank line in the source stays a blank line on screen.
			lines = append(lines, "")
			continue
		}

		current := words[0]
		for _, word := range words[1:] {
			if runeLen(current)+1+runeLen(word) <= width {
				current += " " + word
				continue
			}
			lines = append(lines, current)
			current = word
		}
		lines = append(lines, current)
	}

	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

// runeLen is the number of characters in s, an approximation of how many
// terminal cells it occupies (see wrap).
func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}
