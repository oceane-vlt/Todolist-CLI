package ui

import (
	"fmt"

	todo "github.com/oceane-vlt/todolist/proto"
)

func ShowUi(items []*todo.Item, title string){
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
		completedItemsUi(completed)
	}
}

func notCompletedItemsUi(notCompleted []*todo.Item){
	for i, item := range notCompleted{
		ListItem(i, item.Title, item.Completed)
	}
	
}

func completedItemsUi(completed []*todo.Item){
	for i, item := range completed{
		ListItem(i, item.Title, item.Completed)
	}
}
