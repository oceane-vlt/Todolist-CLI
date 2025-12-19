package main

import (
	"context"
	"fmt"
	"log"

	todo "github.com/oceane-vlt/todolist/proto"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new todo list",
	Long: `Create a new todo list. You can:
  - Create an empty list: todo create mylist
  - Create with items: todo create mylist "Task 1" "Task 2"
  - Create interactively: todo create mylist --interactive`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		todoItems := []*todo.Item{}

		if len(args) > 1 {
			items := args[1:]
			for _, itemTitle := range items {
				todoItem := &todo.Item{
					Title: itemTitle,
				}
				todoItems = append(todoItems, todoItem)
			}
		}
		request := &todo.CreateTodoListRequest{
			Title: args[0],
			Item:  todoItems,
		}

		_, err := grpcClient.CreateTodoList(ctx, request)
		if err != nil {
			log.Fatalf("Error calling CreateTodoList: %v", err)
		}

		fmt.Printf("Todo list created: %v\n", request.Title)
		items := args[1:]
		for _, itemTitle := range items {
			fmt.Printf("- %s\n", itemTitle)
		}
	},
}

func init() {
	createCmd.Flags().StringP("interactive", "i", "", "Create a new todo list in interactive mode to add items")
	rootCmd.AddCommand(createCmd)
}
