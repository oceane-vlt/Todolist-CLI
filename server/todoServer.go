package server

import (
	"context"
	"fmt"

	"github.com/oceane-vlt/todolist/libs"
	todo "github.com/oceane-vlt/todolist/proto"
)

type TodoListServer struct {
	todo.UnimplementedTodoListServiceServer
}

func (s *TodoListServer) CreateTodoList(context.Context, *todo.CreateTodoListRequest) (*todo.CreateTodoListResponse, error) {
	fmt.Println("CreateTodoList")
	return nil, nil
}

func (s *TodoListServer) GetTodoLists(context.Context, *todo.GetTodoListsRequest) (*todo.GetTodoListsResponse, error) {
	fmt.Println("GetTodoLists called")

	lists := libs.GetTodoListsTitles("./data/data.json")

	res := &todo.GetTodoListsResponse{
		Lists: lists,
	}

	return res, nil

}
