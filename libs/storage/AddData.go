package storage

import (
	"encoding/json"
	"fmt"
	"log"

	todo "github.com/oceane-vlt/todolist/proto"
)

func updateData(listToUpdate []TodoItem, newItems []*todo.Item) ([]TodoItem, error) {
	for _, item := range newItems {
		todoItem := TodoItem{
			Title: item.Title,
		}
		listToUpdate = append(listToUpdate, todoItem)
	}
	return listToUpdate, nil
}

func UpdateTodoListData(dataPath, title string, newItems []*todo.Item) error {
	data, err := ReadTodoData(dataPath)
	if err != nil {
		log.Fatal(err)
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
