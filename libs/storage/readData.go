package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	todo "github.com/oceane-vlt/todolist/proto"
)


func GetTodoListsTitles(path string) []*todo.ListSize {
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

func parseTodoListNames(jsonData []byte) []*todo.ListSize {
	var todoData TodoData

	err := json.Unmarshal(jsonData, &todoData)
	if err != nil {
		fmt.Println("Error unmarshaling JSON:", err)
		return nil
	}

	res := []*todo.ListSize{}

	for listName := range todoData.Lists {
		list := todo.ListSize{
			Title: listName,
			Size:  int32(len(todoData.Lists[listName])),
		}
		res = append(res, &list)
		fmt.Printf("Found list:%s size:%d\n", list.Title, list.Size)
	}

	return res
}
