package main

import (
	"context"
	"fmt"
	"os"

	"github.com/oceane-vlt/todolist/libs/errors"
	todo "github.com/oceane-vlt/todolist/proto"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the items of a todo list",
	Long: `Show the items of a todo list. Usage:
  - Show a list: todo show mylist
  - Show a list with details: todo show mylist --verbose`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		request := &todo.ShowTodoListItemsRequest{
			Title: args[0],
		}

		response, err := grpcClient.ShowTodoListItems(ctx, request)
		if err != nil {
			errors.Showerrors(err, args)
			os.Exit(1)
		}

		fmt.Printf("Items in todo list: %v\n", request.Title)
		if verbose, _ := cmd.Flags().GetBool("verbose"); verbose {
			for _, item := range response.Items {
				fmt.Printf("- %s (Description: %s, Completed: %v, DueDate: %s, Priority: %s)\n",
					item.Title, item.Description, item.Completed, item.DueDate, item.Priority)
			}
		} else {
			for _, item := range response.Items {
				fmt.Printf("- %s\n", item.Title)
			}
		}
	},
}

func init() {
	showCmd.Flags().BoolP("verbose", "v", false, "enable verbose output")
	rootCmd.AddCommand(showCmd)
}
