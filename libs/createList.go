package libs

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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

	byteValue, _ := ioutil.ReadAll(jsonFile)

	var todoData TodoData

	err = json.Unmarshal(byteValue, &todoData)
	if err != nil {
		fmt.Println("Error unmarshaling JSON:", err)
		return err
	}

	_, exist := todoData.Lists[title]
	if exist {
		return fmt.Errorf("A todo list with named %s already exists.", title)
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
	ioutil.WriteFile(path, updatedData, 0644)
	return nil
}
