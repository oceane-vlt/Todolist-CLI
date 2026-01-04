package storage

import (
	"encoding/json"
	"fmt"
)

func DeleteTodoList(dataPath string, title []string) error {

	todoData, err := ReadTodoData(dataPath)
	if err != nil {
		return err
	}

	for _, title := range title {
		if findListKey(todoData, title) == "" {
			availableLists := displayList(todoData)
			return fmt.Errorf("todo list \"%s\" does not exist. Available lists:\n%s", title, availableLists)
		}
	}
	for _, title := range title {
		delete(todoData.Lists, title)
	}

	updatedData, err := json.MarshalIndent(todoData, "", "  ")
	if err != nil {
		return err
	}

	if err := WriteTodoData(dataPath, updatedData); err != nil {
		return err
	}

	return nil
}
