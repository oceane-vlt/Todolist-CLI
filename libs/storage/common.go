package storage

import (
	"encoding/json"
	"io"
	"os"
)

type TodoData struct {
	Lists map[string][]TodoItem `json:"lists"`
}

type TodoItem struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
	DueDate     string `json:"dueDate"`
	Priority    string `json:"priority"`
}

func ReadTodoData(dataPath string) (*TodoData, error) {
	jsonFile, err := os.Open(dataPath)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()

	bytesData, err := io.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}

	var todoData TodoData
	err = json.Unmarshal(bytesData, &todoData)
	if err != nil {
		return nil, err
	}

	return &todoData, nil
}

func WriteTodoData(dataPath string, data []byte) error {
	if err := os.WriteFile(dataPath, data, 0644); err != nil {
		return err
	}
	return nil
}
