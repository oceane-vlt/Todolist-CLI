package main

import (
	"context"
	"log"

	todo "github.com/oceane-vlt/todolist/proto"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "update a todo lists with new items",
	Long: `Update a todo list. 
	Usage:
   - Add items to the list: todo update mylist "item1" "My item2" "my last item3"`,
	Args: cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		items := args[1:]
		todoItems := []*todo.Item{}
		for _, itemTitle := range items {
			todoItem := &todo.Item{
				Title: itemTitle,
			}
			todoItems = append(todoItems, todoItem)
		}
		request := &todo.UpdateTodoListRequest{
			Title: args[0],
			Items: todoItems,
		}
		_, err := grpcClient.UpdateTodoList(ctx, request)
		if err != nil {
			log.Fatalf("Error calling UpdateTodoList: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
