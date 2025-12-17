package storage

import (
	"encoding/json"
	"fmt"
	"log"

	todo "github.com/oceane-vlt/todolist/proto"
)

func updateData(data *TodoData, title string, newItems []*todo.Item) ([]TodoItem, error) {
	list, exist := data.Lists[title]
	if !exist {
		return nil, fmt.Errorf("list %s don't exist", title)
	}

	for _, item := range newItems {
		todoItem := TodoItem{
			Title: item.Title,
		}
		list = append(list, todoItem)
	}
	return list, nil
}

func UpdateTodoListData(dataPath, title string, newItems []*todo.Item) error {
	data, err := ReadTodoData(dataPath)
	if err != nil{
		log.Fatal(err)
	}

	updatedList, err := updateData(data, title, newItems)
	if err != nil{
		return err
	}

	data.Lists[title] = updatedList

	updatedData, err := json.MarshalIndent(data, "", " ")
	if err != nil{
		return err
	}

	err = WriteTodoData(dataPath, updatedData)
	if err != nil{
		return err
	}

	return nil
}
