package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func readTodoData(dataPath string) (*TodoData, error) {
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

func writeTodoData(dataPath string, data []byte) error {
	if err := os.WriteFile(dataPath, data, 0644); err != nil {
		return err
	}
	return nil
}

func DeleteTodoList(dataPath string, title string) error {
	
	todoData, err := readTodoData(dataPath)
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

	if err := writeTodoData(dataPath, updatedData); err != nil {
		return err
	}

	return nil
}
