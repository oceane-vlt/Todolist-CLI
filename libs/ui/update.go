package ui

import (
	"fmt"

	todo "github.com/oceane-vlt/todolist/proto"
)

func notCompletedItemsUiList(items []*todo.Item){
	for i, item := range items{
		fmt.Printf(" %d. %s %s%s\n", i+1, ColorReset, item.Title, ColorReset)
	}
}

func UpdateUi(items []*todo.Item, title string) []int32{
	completed := []*todo.Item{}
	notCompleted := []*todo.Item{}

	mapping := []int32{}
	for i, item := range items {
		if item.Completed {
			completed = append(completed, item)
		} else {
			notCompleted = append(notCompleted, item)
			mapping = append(mapping, int32(i))
		}
	}
	
	if len(notCompleted) > 0 {
		fmt.Printf("\n%s To do (%d):%s\n\n", Bold, len(notCompleted), ColorReset)
		notCompletedItemsUiList(notCompleted)
	}
	return mapping
}	
