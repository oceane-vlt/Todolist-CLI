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

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the items of a todo list",
	Long: `Show the items of a todo list. Usage:
  - Show a list (first 7 incomplete items): todo show mylist
  - Show full history (all items): todo show mylist -H
  - Show a list with details: todo show mylist --verbose`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		flagHistory, _ := cmd.Flags().GetBool("history")

		request := &todo.ShowTodoListItemsRequest{
			Title: args[0],
		}

		response, err := grpcClient.ShowTodoListItems(ctx, request)
		if err != nil {
			errors.Showerrors(err, args)
			os.Exit(1)
		}

		if len(response.Items) == 0 {
			ui.Info(fmt.Sprintf("List '%s' is empty. Add items with %s", request.Title, ui.Command("todo add "+request.Title+" <item>")))
			return
		}

		ui.ShowUi(response.Items, request.Title, flagHistory)
		fmt.Println()
	},
}

func init() {
	showCmd.Flags().BoolP("verbose", "v", false, "enable verbose output")
	showCmd.Flags().BoolP("history", "H", false, "Show full completed items history")
	rootCmd.AddCommand(showCmd)
}
