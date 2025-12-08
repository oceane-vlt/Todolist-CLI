package server

import (
	"context"
	"fmt"

	todo "github.com/oceane-vlt/todolist/proto"
)

type TodoListServer struct {
	todo.UnimplementedTodoListServiceServer
}

func (s *TodoListServer) CreateTodoList(context.Context, *todo.CreateTodoListRequest) (*todo.CreateTodoListResponse, error) {
	fmt.Println("CreateTodoList")
	return nil, nil
}

func (s *TodoListServer) GetTodoList(context.Context, *todo.GetTodoListRequest) (*todo.GetTodoListResponse, error) {
	fmt.Println("GetTodoList")
	return nil, nil
}
