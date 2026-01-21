package main

import (
	"context"
	"fmt"

	"os"

	"github.com/oceane-vlt/todolist/libs/errors"
	"github.com/oceane-vlt/todolist/libs/ui"
	todo "github.com/oceane-vlt/todolist/proto"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "add",
	Short: "add items to a todo list",
	Long: `Update a todo list.
	Usage:
   - Add items to the list: todo add mylist "item1" "My item2" "my last item3"`,
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
		updateRequest := &todo.UpdateTodoListRequest{
			Title: args[0],
			Items: todoItems,
		}
		_, err := grpcClient.UpdateTodoList(ctx, updateRequest)
		if err != nil {
			errors.Showerrors(err, args)
			return
		}

		if len(todoItems) == 1 {
			ui.Success(fmt.Sprintf("Added 1 item to '%s'", args[0]))
		} else {
			ui.Success(fmt.Sprintf("Added %d items to '%s'", len(todoItems), args[0]))
		}

		fmt.Println()
		ui.Info("Updated list:")

		request := &todo.ShowTodoListItemsRequest{
			Title: args[0],
		}

		response, err := grpcClient.ShowTodoListItems(ctx, request)
		if err != nil {
			errors.Showerrors(err, args)
			os.Exit(1)
		}
		ui.CompleteUi(response.Items, request.Title)
		fmt.Println()
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
