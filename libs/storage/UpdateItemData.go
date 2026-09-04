package storage

import (
	"encoding/json"
)

// UpdateTodoListItemData applies a partial edit to the item at itemIndex in the
// named list. Only the fields set in update are written; the others keep their
// current value, so editing a description never clobbers the title.
//
// Unknown list and out-of-range index remain silent no-ops (returning nil), as
// they have always been in this backend; PgStore mirrors that exactly so the two
// stay interchangeable behind Store.
func UpdateTodoListItemData(dataPath, title string, itemIndex int32, update ItemUpdate) error {
	if update.IsEmpty() {
		return nil
	}

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

	item := &data.Lists[key][itemIndex]
	if update.Title != nil {
		item.Title = *update.Title
	}
	if update.Description != nil {
		item.Description = *update.Description
	}

	updatedData, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		return err
	}

	return WriteTodoData(dataPath, updatedData)
}
