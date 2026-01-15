package main

import (
	"context"
	"fmt"

	"github.com/oceane-vlt/todolist/libs/errors"
	"github.com/oceane-vlt/todolist/libs/ui"
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
			errors.Showerrors(err, args)
			return
		}

		if len(titles) == 1 {
			ui.Success(fmt.Sprintf("Deleted list '%s'", titles[0]))
		} else {
			ui.Success(fmt.Sprintf("Deleted %d lists", len(titles)))
			for _, title := range titles {
				fmt.Printf("  • %s\n", title)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
