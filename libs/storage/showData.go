package storage

import (
	"fmt"

	todo "github.com/oceane-vlt/todolist/proto"
)

func ShowTodoListItems(dataPath string, title string) ([]*todo.Item, error) {
	todoData, err := ReadTodoData(dataPath)
	if err != nil {
		return nil, err
	}

	actualKey := findListKey(todoData, title)
	if actualKey == "" {
		return nil, fmt.Errorf("todo list %s does not exist", title)
	}

	items := todoData.Lists[actualKey]

	res := []*todo.Item{}
	for _, item := range items {
		todoItem := &todo.Item{
			Title:       item.Title,
			Description: item.Description,
			Completed:   item.Completed,
			DueDate:     item.DueDate,
			Priority:    item.Priority,
		}
		res = append(res, todoItem)
	}

	return res, nil
}
