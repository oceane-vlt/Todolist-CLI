package storage

import (
	"encoding/json"
	"fmt"

	todo "github.com/oceane-vlt/todolist/proto"
)

// updateData appends newItems to listToUpdate. An appended item carries its
// title and its description; the remaining fields keep their zero value until
// the CLI can set them. PgStore.UpdateTodoListData mirrors this exactly.
func updateData(listToUpdate []TodoItem, newItems []*todo.Item) ([]TodoItem, error) {
	for _, item := range newItems {
		todoItem := TodoItem{
			Title:       item.Title,
			Description: item.Description,
		}
		listToUpdate = append(listToUpdate, todoItem)
	}
	return listToUpdate, nil
}

func UpdateTodoListData(dataPath, title string, newItems []*todo.Item) error {
	data, err := ReadTodoData(dataPath)
	if err != nil {
		return err
	}

	key := findListKey(data, title)
	if key == "" {
		return fmt.Errorf("list %s don't exist", title)
	}
	updatedList, err := updateData(data.Lists[key], newItems)
	if err != nil {
		return err
	}

	data.Lists[key] = updatedList

	updatedData, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		return err
	}

	err = WriteTodoData(dataPath, updatedData)
	if err != nil {
		return err
	}

	return nil
}
