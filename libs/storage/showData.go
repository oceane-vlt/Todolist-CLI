package storage

import todo "github.com/oceane-vlt/todolist/proto"

func ShowTodoListItems(dataPath string, title string) ([]*todo.Item, error) {
	todoData, err := ReadTodoData(dataPath)
	if err != nil {
		return nil, err
	}

	items, exist := todoData.Lists[title]
	if !exist {
		return nil, nil
	}

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
