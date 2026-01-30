package ui

import (
	"fmt"

	todo "github.com/oceane-vlt/todolist/proto"
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

func ShowUi(items []*todo.Item, title string, showHistory bool) {
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
		notCompletedItemsUi(notCompleted)
	} else if len(completed) > 0 {
		fmt.Printf("\n%sCongratulations! All items completed in \"%s\" todolist%s\n", BoldGreen, title, ColorReset)
	}

	if len(completed) > 0 {
		fmt.Printf("\n----------------------\n\n")
		fmt.Printf("%s[✓] Completed (%d):%s\n\n", BoldGreen, len(completed), ColorReset)
		completedItemsUi(completed, showHistory)
	}
}

func notCompletedItemsUi(notCompleted []*todo.Item) {
	for i, item := range notCompleted {
		ListItem(i, item.Title, item.Completed)
	}

}

func completedItemsUi(completed []*todo.Item, showHistory bool) {
	if !showHistory {
		// Show only first 7 items
		limit := min(7, len(completed))
		for i := range limit {
			ListItem(i, completed[i].Title, completed[i].Completed)
		}
		if len(completed) > 7 {
			fmt.Printf("\n  %s... run with %s to see full history%s\n", Italic, Command("-H"), ColorReset)
		}
		return
	}
	// Show all items
	for i, item := range completed {
		ListItem(i, item.Title, item.Completed)
	}
}
