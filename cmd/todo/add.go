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

var addCmd = &cobra.Command{
	Use:   "add <list> <item> [item...]",
	Short: "add items to a todo list",
	Long: `Add items to a todo list.
	Usage:
   - Add items to the list: todo add mylist "item1" "My item2" "my last item3"
   - Add one item with a description: todo add mylist "Call the plumber" -d "Leak under the kitchen sink"

The description is the long form of an item; the title stays short. 'todo show'
lists titles only and marks the items that carry a description.`,
	Args: cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		description, _ := cmd.Flags().GetString("description")

		items := args[1:]
		// A description describes ONE item, so pairing it with several titles is
		// ambiguous: refuse rather than silently attaching it to an arbitrary one.
		if description != "" && len(items) > 1 {
			ui.Error("--description applies to a single item.")
			ui.Info(fmt.Sprintf("Add them one at a time, or add the descriptions afterwards: %s, then press e on an item.",
				ui.Command("todo show "+args[0])))
			os.Exit(1)
		}

		todoItems := []*todo.Item{}
		for _, itemTitle := range items {
			todoItem := &todo.Item{
				Title:       itemTitle,
				Description: description,
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
		ui.PendingList(response.Items)
		fmt.Println()
	},
}

func init() {
	addCmd.Flags().StringP("description", "d", "", "Longer description for the item (single item only)")
	rootCmd.AddCommand(addCmd)
}
