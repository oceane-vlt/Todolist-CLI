package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/oceane-vlt/todolist/libs/ui"
)

// Why this is a Bubble Tea form rather than a sequence of prompts:
// promptui repaints its prompt by moving the cursor up once per *logical* line,
// so a prompt whose label plus value exceeds the terminal width — which a
// description does routinely — is wrapped onto two physical rows, repainted one
// row too low, and leaves a copy of itself behind on every keystroke. Its
// ScreenBuf also rejects "\n" outright, so it cannot represent a multi-line
// value at all. A text area is the control long-form text actually needs.

// The form is a SUB-MODEL, not a program of its own: it never returns tea.Quit.
// It raises `done` and lets the browser decide what happens next — which is what
// allows saving an edit to land the user back in the list instead of dropping
// them out of the application.

// Field indices, in tab order.
const (
	fieldTitle = iota
	fieldDescription
	fieldCount
)

// Description body sizing. The form renders inline (like the browser), so the
// whole frame has to fit the terminal; the text area is the only elastic part.
const (
	minDescriptionHeight = 3
	maxDescriptionHeight = 12
)

// editChromeLines is the number of rows the roomy layout spends on anything
// other than the description body, and so what is subtracted from the terminal
// height to size that body. Exact count, because inline rendering breaks on
// overflow:
//
//	1  leading blank line
//	1  the heading ("Edit item" / "New item")
//	1  blank line
//	1  "Title" label
//	1  the title input
//	1  blank line
//	1  "Description" label
//	1  blank line
//	1  key hints
//	1  validation message (always reserved, so the frame does not grow when it
//	   appears and push the form past the bottom of the screen)
const editChromeLines = 10

// compactChromeLines is the same count for the compact layout, used when the
// terminal is too short for the roomy one (a split pane, a small window). The
// title, the breathing room and one of the two bottom rows are given up — the
// message and the hints share a line — while the labels and both fields stay:
//
//	1  "Title" label
//	1  the title input
//	1  "Description" label
//	1  key hints, or the validation message when there is one
//
// Degrading like this rather than overflowing matters: an overflowing frame does
// not merely look cramped, it corrupts the display, because the renderer's
// cursor arithmetic assumes the frame fits.
const compactChromeLines = 4

// minFormHeight is the shortest terminal the form can render into
// (compactChromeLines plus a single-line description area). Below that the frame
// cannot fit and the layout stops shrinking.
const minFormHeight = compactChromeLines + 1

// descriptionCharLimit caps a description generously rather than not at all: an
// unbounded field invites a paste that no longer fits any view that renders it.
const descriptionCharLimit = 2000

type editModel struct {
	// heading names what the form is for ("Edit item" / "New item"), because the
	// same form serves both and the two must not be mistaken for each other.
	heading    string
	titleInput textinput.Model
	descArea   textarea.Model
	focus      int
	saved      bool
	// done reports that the user finished with the form; saved says whether that
	// was a save or a cancel. The owner reads both and resets them.
	done bool
	// message carries a validation problem to show under the form.
	message string
	width   int
	height  int
	// compact drops the form's chrome to fit a short terminal (see
	// compactChromeLines).
	compact bool
}

func newEditModel(heading, title, description string) editModel {
	ti := textinput.New()
	ti.SetValue(title)
	ti.Prompt = ""
	ti.CursorEnd()
	ti.Focus()

	ta := textarea.New()
	ta.SetValue(description)
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.CharLimit = descriptionCharLimit
	ta.Placeholder = "No description. Type one, or leave empty."
	ta.Blur()

	m := editModel{
		heading:    heading,
		titleInput: ti,
		descArea:   ta,
		focus:      fieldTitle,
		width:      defaultWidth,
		height:     defaultHeight,
	}
	m.layout()
	return m
}

// update advances the form. It returns a concrete editModel rather than a
// tea.Model so the browser can embed it without casting on every keystroke.
func (m editModel) update(msg tea.Msg) (editModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()
		return m, nil

	case tea.KeyMsg:
		// These are handled before the focused component sees the key, so the
		// text area cannot swallow Esc as "stop selecting" or Tab as an indent.
		//
		// Ctrl+C is absent on purpose: the browser intercepts it globally, so an
		// interrupt leaves the application rather than merely closing the form.
		switch msg.String() {
		case "esc":
			m.done, m.saved = true, false
			return m, nil

		case "ctrl+s":
			if strings.TrimSpace(m.titleInput.Value()) == "" {
				m.message = "A title is required."
				return m, nil
			}
			m.done, m.saved = true, true
			return m, nil

		case "tab":
			return m.focusField(m.focus + 1), nil

		case "shift+tab":
			return m.focusField(m.focus - 1), nil

		case "enter":
			// On a single-line field Enter means "done with this field"; inside the
			// description it has to stay a newline, which is the point of the area.
			if m.focus == fieldTitle {
				return m.focusField(fieldDescription), nil
			}
		}

		// Any edit clears a stale validation message.
		m.message = ""
	}

	var cmd tea.Cmd
	if m.focus == fieldTitle {
		m.titleInput, cmd = m.titleInput.Update(msg)
	} else {
		m.descArea, cmd = m.descArea.Update(msg)
	}
	return m, cmd
}

// values returns the edited title and description, trimmed. TrimSpace also drops
// the leading/trailing blank lines a multi-line edit tends to leave behind,
// while keeping the blank lines inside the text.
func (m editModel) values() (title, description string) {
	return strings.TrimSpace(m.titleInput.Value()), strings.TrimSpace(m.descArea.Value())
}

// focusField moves the focus, wrapping around, and keeps exactly one component
// focused so only one cursor blinks.
func (m editModel) focusField(next int) editModel {
	m.focus = ((next % fieldCount) + fieldCount) % fieldCount

	if m.focus == fieldTitle {
		m.titleInput.Focus()
		m.descArea.Blur()
	} else {
		m.titleInput.Blur()
		m.descArea.Focus()
	}
	return m
}

// layout resizes the components to the current terminal, sizing the description
// body with whatever height is left once the fixed chrome is accounted for.
func (m *editModel) layout() {
	width := m.width - 2*len(rowIndent)
	if width < minContentWidth {
		width = minContentWidth
	}
	m.titleInput.Width = width
	m.descArea.SetWidth(width)

	// Prefer the roomy layout, and fall back to the compact one only when the
	// roomy chrome plus a usable description area would not fit.
	m.compact = m.height < editChromeLines+minDescriptionHeight

	chrome := editChromeLines
	if m.compact {
		chrome = compactChromeLines
	}

	height := m.height - chrome
	if height < 1 {
		// Terminal shorter than minFormHeight: render the smallest frame we can.
		height = 1
	}
	if !m.compact && height < minDescriptionHeight {
		height = minDescriptionHeight
	}
	if height > maxDescriptionHeight {
		height = maxDescriptionHeight
	}
	m.descArea.SetHeight(height)
}

// editHint spells the modifier out as "ctrl+s" rather than the macOS glyph
// "⌃s": the glyph is only recognisable to users who already know it, and on a
// Mac the instinct is Cmd+S — which a terminal never delivers to a program.
const editHint = "tab next field · ctrl+s save · esc cancel"

func (m editModel) View() string {
	var b strings.Builder

	if !m.compact {
		fmt.Fprintf(&b, "\n%s%s%s%s\n\n", rowIndent, ui.Bold, m.heading, ui.ColorReset)
	}

	b.WriteString(m.label("Title", fieldTitle))
	b.WriteString(indentLines(m.titleInput.View(), rowIndent))
	b.WriteString("\n")
	if !m.compact {
		b.WriteString("\n")
	}

	b.WriteString(m.label("Description", fieldDescription))
	b.WriteString(indentLines(m.descArea.View(), rowIndent))
	b.WriteString("\n")
	if !m.compact {
		b.WriteString("\n")
	}

	if m.compact {
		// One row for both: the message when there is one, the hints otherwise.
		if m.message != "" {
			fmt.Fprintf(&b, "%s%s%s%s\n", rowIndent, ui.BoldRed, m.message, ui.ColorReset)
		} else {
			fmt.Fprintf(&b, "%s%s%s%s\n", rowIndent, ui.Dim, editHint, ui.ColorReset)
		}
		return b.String()
	}

	if m.message != "" {
		fmt.Fprintf(&b, "%s%s%s%s\n", rowIndent, ui.BoldRed, m.message, ui.ColorReset)
	} else {
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "%s%s%s%s\n", rowIndent, ui.Dim, editHint, ui.ColorReset)

	return b.String()
}

// label renders a field label, accented when that field has the focus so the
// active field is identifiable without hunting for the cursor.
func (m editModel) label(text string, field int) string {
	style := ui.Dim
	if m.focus == field {
		style = ui.BoldAccent
	}
	return fmt.Sprintf("%s%s%s%s\n", rowIndent, style, text, ui.ColorReset)
}

// indentLines prefixes every line of s with indent. It is how the components'
// own rendering is placed inside the form's margin without having to style
// their internals.
func indentLines(s, indent string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}
