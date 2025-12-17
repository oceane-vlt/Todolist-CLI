package storage

import (
	"encoding/json"
	"fmt"
	"slices"
)

func contains(item int32, items []int32) bool {
	return slices.Contains(items, item)
}

func deleteItems(items []TodoItem, indicesToDelete []int32) []TodoItem {
	updatedItems := []TodoItem{}
	for i, item := range items {
		if !contains(int32(i), indicesToDelete) {
			updatedItems = append(updatedItems, item)
		}
	}

	return updatedItems
}


func DeleteTodoListItems(dataPath, title string, indicesToDelete []int32) error {
	todoData, err := ReadTodoData(dataPath)
	if err != nil {
		return err
	}

	listItems, exist := todoData.Lists[title]
	if !exist {
		return fmt.Errorf("todo list %s does not exist", title)
	}

	for _, idx := range indicesToDelete {
		if idx < 0 || idx >= int32(len(todoData.Lists[title])) {
			return fmt.Errorf("item index %d out of range for todo list %s", idx, title)
		}
	}

	updatedItems := deleteItems(listItems, indicesToDelete)

	todoData.Lists[title] = updatedItems

	updatedData, err := json.MarshalIndent(todoData, "", "  ")
	if err != nil {
		return err
	}

	if err := WriteTodoData(dataPath, updatedData); err != nil {
		return err
	}

	return nil
}
