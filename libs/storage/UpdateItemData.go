package storage

import (
	"encoding/json"
)

func UpdateTodoListItemData(dataPath, title string, itemIndex int32, newTitle string) error {
	data, err := ReadTodoData(dataPath)
	if err != nil {
		return err
	}

	key := findListKey(data, title)
	if key == "" {
		return nil
	}

	if int(itemIndex) < 0 || int(itemIndex) >= len(data.Lists[key]) {
		return nil
	}

	data.Lists[key][itemIndex].Title = newTitle

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
