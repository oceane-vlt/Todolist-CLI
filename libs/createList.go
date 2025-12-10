package libs

import (
	todo "github.com/oceane-vlt/todolist/proto"
	"fmt"
	"os"
	"encoding/json"
	"io/ioutil"
)


func CreateTodoList(path string, title string, items []*todo.Item) {
	jsonFile, err := os.Open(path)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer jsonFile.Close()

	fmt.Printf("Successfully Opened %s", path)

	byteValue, _ := ioutil.ReadAll(jsonFile)

	var todoData TodoData

	err = json.Unmarshal(byteValue, &todoData)
	if err != nil {
		fmt.Println("Error unmarshaling JSON:", err)
		return
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
		return
	}
	ioutil.WriteFile(path, updatedData, 0644)
}
