package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type TodoData struct {
	Lists map[string][]TodoItem `json:"lists"`
}

type TodoItem struct {
	Id          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
	DueDate     string `json:"dueDate"`
	Priority    string `json:"priority"`
}

func GetTodoListsTitles(path string) []string {
	jsonFile, err := os.Open(path)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	defer jsonFile.Close()

	fmt.Println("Successfully Opened data.json")

	bytesData, err := io.ReadAll(jsonFile)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return nil
	}

	return parseTodoListNames(bytesData)
}

func parseTodoListNames(jsonData []byte) []string {
	var todoData TodoData

	err := json.Unmarshal(jsonData, &todoData)
	if err != nil {
		fmt.Println("Error unmarshaling JSON:", err)
		return nil
	}

	// Extract list names (keys of the map)
	var listNames []string
	for listName := range todoData.Lists {
		listNames = append(listNames, listName)
		fmt.Println("Found list:", listName)
	}

	return listNames
}
