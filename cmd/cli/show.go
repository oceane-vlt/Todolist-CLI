package main

import (
	"context"
	"fmt"
	"log"

	todo "github.com/oceane-vlt/todolist/proto"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the items of a todo list",
	Long: `Show the items of a todo list. Usage:
  - Show a list: todo show mylist`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		request := &todo.ShowTodoListItemsRequest{
			Title: args[0],
		}

		response, err := grpcClient.ShowTodoListItems(ctx, request)
		if err != nil {
			log.Fatalf("Error calling ShowTodoListItems: %v", err)
		}

		fmt.Printf("Items in todo list: %v\n", request.Title)
		for _, item := range response.Items {
			fmt.Printf("- %s (Description: %s, Completed: %v, DueDate: %s, Priority: %s)\n",
				item.Title, item.Description, item.Completed, item.DueDate, item.Priority)
		}
	},
}

func init() {
	rootCmd.AddCommand(showCmd)
}
