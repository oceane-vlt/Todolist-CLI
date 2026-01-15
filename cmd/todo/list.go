package main

import (
	"context"
	"fmt"

	"github.com/oceane-vlt/todolist/libs/errors"
	"github.com/oceane-vlt/todolist/libs/ui"
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
			errors.Showerrors(err, args)
			return
		}

		if len(response.Lists) == 0 {
			ui.Info("No todo lists yet. Create one with " + ui.Command("todo create <name>"))
			return
		}

		ui.Header("📋 Your todo lists:")
		for _, list := range response.Lists {
			itemText := "item"
			if list.Size != 1 {
				itemText = "items"
			}
			fmt.Printf("  %s  (%d %s)\n", ui.BoldText(list.Title), list.Size, itemText)
		}
		fmt.Println()
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
