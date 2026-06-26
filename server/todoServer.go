package server

import (
	"context"
	"fmt"

	"github.com/oceane-vlt/todolist/libs/storage"
	todo "github.com/oceane-vlt/todolist/proto"
)

// TodoListServer implements the gRPC TodoListService. It depends on the
// storage.Store interface (Phase 0 seam) rather than a concrete backend, so the
// persistence layer can be swapped (JSON today, Postgres later) without
// touching the handlers.
type TodoListServer struct {
	todo.UnimplementedTodoListServiceServer
	store storage.Store
}

// NewTodoListServer builds a TodoListServer backed by the given Store.
func NewTodoListServer(store storage.Store) *TodoListServer {
	return &TodoListServer{store: store}
}

func (s *TodoListServer) CreateTodoList(ctx context.Context, request *todo.CreateTodoListRequest) (*todo.CreateTodoListResponse, error) {
	fmt.Println("CreateTodoList")

	err := s.store.CreateTodoList(ctx, request.Title, request.Item)
	if err != nil {
		return nil, err
	}
	res := &todo.CreateTodoListResponse{}
	return res, nil
}

func (s *TodoListServer) GetTodoLists(ctx context.Context, _ *todo.GetTodoListsRequest) (*todo.GetTodoListsResponse, error) {
	fmt.Println("GetTodoLists called")

	lists := s.store.GetTodoListsTitles(ctx)

	res := &todo.GetTodoListsResponse{
		Lists: lists,
	}

	return res, nil
}

func (s *TodoListServer) DeleteTodoList(ctx context.Context, request *todo.DeleteTodoListRequest) (*todo.DeleteTodoListResponse, error) {
	fmt.Println("DeleteTodoList called")

	err := s.store.DeleteTodoList(ctx, request.Title)
	if err != nil {
		return nil, err
	}
	res := &todo.DeleteTodoListResponse{}
	return res, nil
}

func (s *TodoListServer) ShowTodoListItems(ctx context.Context, request *todo.ShowTodoListItemsRequest) (*todo.ShowTodoListItemsResponse, error) {
	fmt.Println("ShowTodoListItems called")

	items, err := s.store.ShowTodoListItems(ctx, request.Title)
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

	err := s.store.DeleteTodoListItems(ctx, request.Title, request.ItemIndexes)
	if err != nil {
		return nil, err
	}
	res := &todo.DeleteTodoListItemsResponse{}
	return res, nil
}

func (s *TodoListServer) UpdateTodoList(ctx context.Context, request *todo.UpdateTodoListRequest) (*todo.UpdateTodoListResponse, error) {
	fmt.Println("UpdateTodoList called")

	err := s.store.UpdateTodoListData(ctx, request.Title, request.Items)
	if err != nil {
		return nil, err
	}
	res := &todo.UpdateTodoListResponse{}
	return res, nil
}

func (s *TodoListServer) UpdateTodoListItem(ctx context.Context, request *todo.UpdateTodoListItemRequest) (*todo.UpdateTodoListItemResponse, error) {
	fmt.Println("UpdateTodoListItem called")

	err := s.store.UpdateTodoListItemData(ctx, request.Title, request.ItemIndex, request.NewTitle)
	if err != nil {
		return nil, err
	}
	res := &todo.UpdateTodoListItemResponse{}
	return res, nil
}
