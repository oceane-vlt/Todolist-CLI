package main

import (
	"context"
	"fmt"
	"log"

	todo "github.com/oceane-vlt/todolist/proto"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Return all todo lists",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		response, err := grpcClient.GetTodoLists(ctx, &todo.GetTodoListsRequest{})
		if err != nil {
			log.Fatalf("Error calling GetTodoList: %v", err)
		}

		for _, list := range response.Lists {
			fmt.Printf("- %s, size: %d\n", list.Title, list.Size)
		}

	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
