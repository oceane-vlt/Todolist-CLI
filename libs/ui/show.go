package ui

import (
	"fmt"
	"strings"

	todo "github.com/oceane-vlt/todolist/proto"
)

// descriptionMarker flags an item that carries a description. The static
// rendering shows titles only, so without this column a description would be
// invisible — and unreachable, since the reader would not know to ask for it.
const descriptionMarker = "▸"

// Column widths of a static item row, in terminal cells:
//
//	␣␣ │ index(2) ". " │ marker ␣ │ checkbox ␣ │ title
//
// The index is right-aligned in two cells so rows stay aligned past item 9, and
// descriptionColumn is derived from the same widths so a wrapped description
// lines up under the title it belongs to instead of floating left of it.
const (
	staticIndent      = "  "
	indexWidth        = 2
	markerWidth       = 1
	checkboxWidth     = 3
	descriptionColumn = len(staticIndent) + indexWidth + 2 + markerWidth + 1 + checkboxWidth + 1
)

// ListItem prints a todo item with checkbox
func ListItem(index int, title string, completed bool) {
	checkbox := "[ ]"
	color := ColorReset
	if completed {
		checkbox = "[✓]"
		color = ColorGray
	}
	fmt.Printf("  %d. %s%s %s%s\n", index+1, color, checkbox, title, ColorReset)
}

// itemLine prints one item, adding the description marker and, when showDetails
// is set, the description itself indented underneath.
//
// The styling matches the interactive browser (libs/tui): the title keeps the
// terminal's default colour because it is the content, while the index, the
// marker and the checkbox are dimmed because they are structure.
func itemLine(index int, item *todo.Item, showDetails bool) {
	checkbox := "[ ]"
	titleStyle := ""
	if item.Completed {
		checkbox = "[✓]"
		titleStyle = Dim
	}

	marker := " "
	if hasDescription(item) {
		marker = descriptionMarker
	}

	fmt.Printf("%s%s%*d.%s %s%s%s %s%s%s %s%s%s\n",
		staticIndent,
		Dim, indexWidth, index+1, ColorReset,
		Dim, marker, ColorReset,
		Dim, checkbox, ColorReset,
		titleStyle, item.Title, ColorReset)

	if showDetails && hasDescription(item) {
		indent := strings.Repeat(" ", descriptionColumn)
		for _, line := range strings.Split(strings.TrimSpace(item.Description), "\n") {
			fmt.Printf("%s%s%s%s\n", indent, Italic, strings.TrimSpace(line), ColorReset)
		}
	}
}

// hasDescription reports whether an item carries a non-blank description.
func hasDescription(item *todo.Item) bool {
	return strings.TrimSpace(item.Description) != ""
}

// anyHasDescription reports whether at least one item carries a description,
// used to decide whether the marker legend is worth printing.
func anyHasDescription(items []*todo.Item) bool {
	for _, item := range items {
		if hasDescription(item) {
			return true
		}
	}
	return false
}

// ShowUi renders a list statically: titles only, with a marker on the items
// that have a description, and the descriptions themselves when showDetails is
// set. This is the rendering used whenever the interactive browser cannot run
// (a pipe, a script, or --plain).
func ShowUi(items []*todo.Item, title string, showHistory, showDetails bool) {
	completed := []*todo.Item{}
	notCompleted := []*todo.Item{}

	for _, item := range items {
		if item.Completed {
			completed = append(completed, item)
		} else {
			notCompleted = append(notCompleted, item)
		}
	}

	if len(notCompleted) > 0 {
		fmt.Printf("\n%s[ ] To do (%d):%s\n\n", Bold, len(notCompleted), ColorReset)
		notCompletedItemsUi(notCompleted, showDetails)
	} else if len(completed) > 0 {
		fmt.Printf("\n%sCongratulations! All items completed in \"%s\" todolist%s\n", BoldGreen, title, ColorReset)
	}

	if len(completed) > 0 {
		fmt.Printf("\n----------------------\n\n")
		fmt.Printf("%s[✓] Completed (%d):%s\n\n", BoldGreen, len(completed), ColorReset)
		completedItemsUi(completed, showHistory, showDetails)
	}

	if !showDetails && anyHasDescription(items) {
		fmt.Printf("\n%s%s%s marks an item with a description — see it with %s%s\n",
			staticIndent, Dim, descriptionMarker, Command("--details"), ColorReset)
	}
}

func notCompletedItemsUi(notCompleted []*todo.Item, showDetails bool) {
	for i, item := range notCompleted {
		itemLine(i, item, showDetails)
	}
}

func completedItemsUi(completed []*todo.Item, showHistory, showDetails bool) {
	if !showHistory {
		// Show only first 7 items
		limit := min(7, len(completed))
		for i := range limit {
			itemLine(i, completed[i], showDetails)
		}
		if len(completed) > 7 {
			fmt.Printf("\n  %s... run with %s to see full history%s\n", Italic, Command("-H"), ColorReset)
		}
		return
	}
	// Show all items
	for i, item := range completed {
		itemLine(i, item, showDetails)
	}
}

// PendingList prints the not-completed items of a list, numbered from 1, using
// the same row rendering as ShowUi. It is what a mutating command (such as
// "todo add") uses to echo the resulting list back to the user.
//
// Completed items are left out on purpose: the echo is there to confirm what is
// still to do, and the history has its own flag on "todo show".
func PendingList(items []*todo.Item) {
	pending := make([]*todo.Item, 0, len(items))
	for _, item := range items {
		if !item.Completed {
			pending = append(pending, item)
		}
	}
	for i, item := range pending {
		itemLine(i, item, false)
	}
}
