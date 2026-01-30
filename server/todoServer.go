package server

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/oceane-vlt/todolist/libs/storage"
	todo "github.com/oceane-vlt/todolist/proto"
)

var path string

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Failed to get user home directory: %v", err)
	}

	// Create config directory if it doesn't exist
	configDir := filepath.Join(home, ".config", "todolist")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		log.Fatalf("Failed to create config directory: %v", err)
	}

	path = filepath.Join(configDir, "data.json")

	// Create initial data file if it doesn't exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		initialData := []byte(`{"lists":{}}`)
		if err := os.WriteFile(path, initialData, 0644); err != nil {
			log.Fatalf("Failed to create initial data file: %v", err)
		}
		log.Printf("Created initial data file at: %s", path)
	}

	log.Printf("Using data file: %s", path)
}

type TodoListServer struct {
	todo.UnimplementedTodoListServiceServer
}

func (s *TodoListServer) CreateTodoList(ctx context.Context, request *todo.CreateTodoListRequest) (*todo.CreateTodoListResponse, error) {
	fmt.Println("CreateTodoList")

	err := storage.CreateTodoList(path, request.Title, request.Item)
	if err != nil {
		return nil, err
	}
	res := &todo.CreateTodoListResponse{}
	return res, nil
}

func (s *TodoListServer) GetTodoLists(context.Context, *todo.GetTodoListsRequest) (*todo.GetTodoListsResponse, error) {
	fmt.Println("GetTodoLists called")

	lists := storage.GetTodoListsTitles(path)

	res := &todo.GetTodoListsResponse{
		Lists: lists,
	}

	return res, nil

}

func (s *TodoListServer) DeleteTodoList(ctx context.Context, request *todo.DeleteTodoListRequest) (*todo.DeleteTodoListResponse, error) {
	fmt.Println("DeleteTodoList called")

	err := storage.DeleteTodoList(path, request.Title)
	if err != nil {
		return nil, err
	}
	res := &todo.DeleteTodoListResponse{}
	return res, nil
}

func (s *TodoListServer) ShowTodoListItems(ctx context.Context, request *todo.ShowTodoListItemsRequest) (*todo.ShowTodoListItemsResponse, error) {
	fmt.Println("ShowTodoListItems called")

	items, err := storage.ShowTodoListItems(path, request.Title)
	if err != nil {
		return nil, err
	}

	res := &todo.ShowTodoListItemsResponse{
		Items: items,
	}
	return res, nil
}

func (s *TodoListServer) DeleteTodoListItems(ctx context.Context, request *todo.DeleteTodoListItemsRequest) (*todo.DeleteTodoListItemsResponse, error) {
	fmt.Println("DeleteTodoListItems called")

	err := storage.DeleteTodoListItems(path, request.Title, request.ItemIndexes)
	if err != nil {
		return nil, err
	}
	res := &todo.DeleteTodoListItemsResponse{}
	return res, nil
}

func (s *TodoListServer) UpdateTodoList(ctx context.Context, request *todo.UpdateTodoListRequest) (*todo.UpdateTodoListResponse, error) {
	fmt.Println("UpdateTodoList called")

	err := storage.UpdateTodoListData(path, request.Title, request.Items)
	if err != nil {
		return nil, err
	}
	res := &todo.UpdateTodoListResponse{}
	return res, nil
}

func (s *TodoListServer) UpdateTodoListItem(ctx context.Context, request *todo.UpdateTodoListItemRequest) (*todo.UpdateTodoListItemResponse, error) {
	fmt.Println("UpdateTodoListItem called")

	err := storage.UpdateTodoListItemData(path, request.Title, request.ItemIndex, request.NewTitle)
	if err != nil {
		return nil, err
	}
	res := &todo.UpdateTodoListItemResponse{}
	return res, nil
}
