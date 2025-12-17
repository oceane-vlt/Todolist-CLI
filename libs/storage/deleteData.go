package storage

import (
	"encoding/json"
	"fmt"
)

func DeleteTodoList(dataPath string, title string) error {

	todoData, err := ReadTodoData(dataPath)
	if err != nil {
		return err
	}

	_, exist := todoData.Lists[title]
	if !exist {
		return fmt.Errorf("todo list %s does not exist", title)
	}

	delete(todoData.Lists, title)

	updatedData, err := json.MarshalIndent(todoData, "", "  ")
	if err != nil {
		return err
	}

	if err := WriteTodoData(dataPath, updatedData); err != nil {
		return err
	}

	return nil
}
