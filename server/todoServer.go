package server

import (
	"context"
	"fmt"

	"github.com/oceane-vlt/todolist/libs/storage"
	todo "github.com/oceane-vlt/todolist/proto"
)

var path = "./data/data.json"
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
