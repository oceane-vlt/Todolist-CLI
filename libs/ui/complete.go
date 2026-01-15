package ui

import (
	"fmt"

	todo "github.com/oceane-vlt/todolist/proto"
)

func CompleteUi(items []*todo.Item, title string) []int32 {
	// Build mapping from displayed index to actual index
	var mapping []int32
	displayIndex := 1
	for i, item := range items {
		if !item.Completed {
			fmt.Printf("  %d. [ ] %s\n", displayIndex, item.Title)
			mapping = append(mapping, int32(i))
			displayIndex++
		}
	}
	return mapping
}
