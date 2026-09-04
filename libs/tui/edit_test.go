package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// key builds a KeyMsg for a special key.
func key(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }

// typeRunes builds a KeyMsg for typed characters.
func typeRunes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// send drives the form through one message.
func send(t *testing.T, m editModel, msg tea.Msg) editModel {
	t.Helper()
	next, _ := m.update(msg)
	return next
}

func TestEditFocusCycles(t *testing.T) {
	m := newEditModel(headingEdit, "title", "description")
	if m.focus != fieldTitle {
		t.Fatalf("focus starts at %d, want the title field", m.focus)
	}

	m = send(t, m, key(tea.KeyTab))
	if m.focus != fieldDescription {
		t.Errorf("after Tab focus = %d, want the description field", m.focus)
	}

	// Wraps around rather than stopping at the last field.
	m = send(t, m, key(tea.KeyTab))
	if m.focus != fieldTitle {
		t.Errorf("Tab past the last field = %d, want wrap to the title", m.focus)
	}

	// And backwards, which is where a naive modulo goes negative.
	m = send(t, m, key(tea.KeyShiftTab))
	if m.focus != fieldDescription {
		t.Errorf("Shift+Tab before the first field = %d, want wrap to the description", m.focus)
	}
}

// TestEditEnterLeavesTitleButNotDescription pins the asymmetry: Enter is
// "field done" on a single-line input, and a real newline in the text area.
func TestEditEnterLeavesTitleButNotDescription(t *testing.T) {
	m := newEditModel(headingEdit, "title", "one")

	m = send(t, m, key(tea.KeyEnter))
	if m.focus != fieldDescription {
		t.Fatalf("Enter on the title moved focus to %d, want the description", m.focus)
	}

	m = send(t, m, key(tea.KeyEnter))
	if m.focus != fieldDescription {
		t.Errorf("Enter inside the description moved focus to %d, want it to stay", m.focus)
	}
	if !strings.Contains(m.descArea.Value(), "\n") {
		t.Errorf("Enter inside the description should insert a newline, got %q", m.descArea.Value())
	}
}

func TestEditSaveRequiresATitle(t *testing.T) {
	m := newEditModel(headingEdit, "", "a description")

	m = send(t, m, key(tea.KeyCtrlS))
	if m.saved || m.done {
		t.Error("saving with a blank title should be refused")
	}
	if m.message == "" {
		t.Error("refusing to save should explain why")
	}

	// Typing clears the complaint, so it does not linger once fixed.
	m = send(t, m, key(tea.KeyTab))
	m = send(t, m, typeRunes("x"))
	if m.message != "" {
		t.Errorf("message = %q, want it cleared once the user edits again", m.message)
	}
}

// TestEditSaveAndCancel checks both outcomes report `done`, which is how the
// browser knows to close the form and what to do next. Ctrl+C is absent: the
// browser intercepts it before the form ever sees it.
func TestEditSaveAndCancel(t *testing.T) {
	saved := send(t, newEditModel(headingEdit, "title", "desc"), key(tea.KeyCtrlS))
	if !saved.done || !saved.saved {
		t.Errorf("Ctrl+S with a title: done=%v saved=%v, want both true", saved.done, saved.saved)
	}

	cancelled := send(t, newEditModel(headingEdit, "title", "desc"), key(tea.KeyEsc))
	if !cancelled.done || cancelled.saved {
		t.Errorf("Esc: done=%v saved=%v, want done and not saved", cancelled.done, cancelled.saved)
	}

	// A refused save must NOT report done, or the browser would close the form
	// and throw the edit away.
	blank := send(t, newEditModel(headingEdit, "", "desc"), key(tea.KeyCtrlS))
	if blank.done {
		t.Error("a refused save must not report done")
	}
}

// TestEditValuesAreTrimmed covers what the browser actually reads back.
func TestEditValuesAreTrimmed(t *testing.T) {
	m := newEditModel(headingEdit, "  spaced  ", "\n\n  body  \n\n")
	title, description := m.values()
	if title != "spaced" {
		t.Errorf("title = %q, want %q", title, "spaced")
	}
	if description != "body" {
		t.Errorf("description = %q, want %q", description, "body")
	}
}

// TestEditViewFitsTerminalHeight is the same invariant the browser has: the form
// renders inline, so it must never be taller than the terminal.
func TestEditViewFitsTerminalHeight(t *testing.T) {
	long := strings.Repeat("a long description that will need several lines. ", 20)

	// Down to minFormHeight, where the compact layout is at its smallest.
	for _, height := range []int{minFormHeight, 6, 8, 10, 12, 15, 24, 40, 60} {
		for _, desc := range []string{"", "short", long} {
			m := newEditModel(headingEdit, "a title", desc)
			m.height = height
			m.width = 60
			m.layout()

			for _, focus := range []int{fieldTitle, fieldDescription} {
				m = m.focusField(focus)
				lines := strings.Count(m.View(), "\n")
				if lines > height {
					t.Errorf("height=%d desc=%dch focus=%d: form is %d lines, exceeds the terminal",
						height, len(desc), focus, lines)
				}
			}
		}
	}
}

// TestEditLayoutClampsDescriptionHeight covers both ends of the sizing.
//
// minDescriptionHeight is the floor of the ROOMY layout only: the compact one is
// allowed to go below it, down to a single line, because on a short terminal
// fitting the frame matters more than the comfort of the text area — an
// overflowing frame corrupts the display, a one-line one merely scrolls.
func TestEditLayoutClampsDescriptionHeight(t *testing.T) {
	roomy := newEditModel(headingEdit, "t", "d")
	roomy.height = editChromeLines + minDescriptionHeight
	roomy.layout()
	if got := roomy.descArea.Height(); got < minDescriptionHeight {
		t.Errorf("roomy layout gave a %d-line description area, want at least %d", got, minDescriptionHeight)
	}

	tiny := newEditModel(headingEdit, "t", "d")
	tiny.height = 4
	tiny.layout()
	if got := tiny.descArea.Height(); got < 1 {
		t.Errorf("tiny terminal gave a %d-line description area, want at least 1", got)
	}

	huge := newEditModel(headingEdit, "t", "d")
	huge.height = 300
	huge.layout()
	if got := huge.descArea.Height(); got > maxDescriptionHeight {
		t.Errorf("huge terminal gave a %d-line description area, want at most %d", got, maxDescriptionHeight)
	}
}

// TestEditCompactLayoutKicksIn documents where the layout switches, so the two
// chrome counts and the switch condition cannot drift apart unnoticed.
func TestEditCompactLayoutKicksIn(t *testing.T) {
	roomy := newEditModel(headingEdit, "t", "d")
	roomy.height = editChromeLines + minDescriptionHeight
	roomy.layout()
	if roomy.compact {
		t.Errorf("height=%d should still use the roomy layout", roomy.height)
	}

	compact := newEditModel(headingEdit, "t", "d")
	compact.height = editChromeLines + minDescriptionHeight - 1
	compact.layout()
	if !compact.compact {
		t.Errorf("height=%d should fall back to the compact layout", compact.height)
	}
}

// TestEditCompactShowsMessageInsteadOfHints covers the row the compact layout
// saves by sharing one line between the two.
func TestEditCompactShowsMessageInsteadOfHints(t *testing.T) {
	m := newEditModel(headingEdit, "", "d")
	m.height = minFormHeight
	m.layout()
	if !m.compact {
		t.Fatal("expected the compact layout at minFormHeight")
	}

	if !strings.Contains(m.View(), editHint) {
		t.Error("compact form should show the key hints when there is no message")
	}

	m = send(t, m, key(tea.KeyCtrlS))
	view := m.View()
	if !strings.Contains(view, m.message) {
		t.Error("compact form should show the validation message")
	}
	if strings.Contains(view, editHint) {
		t.Error("compact form should give the hints row to the message, not print both")
	}
	if lines := strings.Count(view, "\n"); lines > m.height {
		t.Errorf("compact form with a message is %d lines, exceeds the %d-line terminal", lines, m.height)
	}
}
