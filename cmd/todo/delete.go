package main

import (
	"context"
	"fmt"
	"log"

	todo "github.com/oceane-vlt/todolist/proto"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete one or more todo lists",
	Long: `Delete one or more todo lists. Usage:
  - Delete a single list: todo delete mylist
  - Delete multiple lists: todo delete list1 list2 list3`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		titles := args
		request := &todo.DeleteTodoListRequest{
			Title: titles,
		}

		_, err := grpcClient.DeleteTodoList(ctx, request)
		if err != nil {
			log.Fatalf("Error calling DeleteTodoList: %v", err)
		}
		fmt.Printf("Todo list deleted: %v\n", request.Title)
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
