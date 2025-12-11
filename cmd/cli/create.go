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
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		request := &todo.CreateTodoListRequest{
			Title: args[0],
			Item:  []*todo.Item{},
		}

		_, err := grpcClient.CreateTodoList(ctx, request)
		if err != nil {
			log.Fatalf("Error calling CreateTodoList: %v", err)
		}

		fmt.Printf("Todo list created: %v\n", request.Title)
	},
}

func init() {
	rootCmd.AddCommand(createCmd)
}
