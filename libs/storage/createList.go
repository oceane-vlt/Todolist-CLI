package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	todo "github.com/oceane-vlt/todolist/proto"
)

func CreateTodoList(path string, title string, items []*todo.Item) error {
	jsonFile, err := os.Open(path)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer jsonFile.Close()

	fmt.Printf("Successfully Opened %s\n", path)

	byteValue, _ := io.ReadAll(jsonFile)

	var todoData TodoData

	err = json.Unmarshal(byteValue, &todoData)
	if err != nil {
		fmt.Println("Error unmarshaling JSON:", err)
		return err
	}

	_, exist := todoData.Lists[title]
	if exist {
		return fmt.Errorf("a todo list named %s already exists", title)
	}

	todoData.Lists[title] = []TodoItem{}

	for _, item := range items {
		todoItem := TodoItem{
			Title:       item.Title,
			Description: item.Description,
			Completed:   item.Completed,
			DueDate:     item.DueDate,
			Priority:    item.Priority,
		}
		todoData.Lists[title] = append(todoData.Lists[title], todoItem)
	}
	updatedData, err := json.MarshalIndent(todoData, "", "  ")
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return err
	}
	if err := os.WriteFile(path, updatedData, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	return nil
}
