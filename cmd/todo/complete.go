package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/oceane-vlt/todolist/libs/errors"
	"github.com/oceane-vlt/todolist/libs/ui"
	todo "github.com/oceane-vlt/todolist/proto"
	"github.com/spf13/cobra"
)

var completeCmd = &cobra.Command{
	Use:   "complete",
	Short: "Mark items as completed in a todo list",
	Long: `Mark items as completed in a todo list by specifying the list title and item indices.
You can provide multiple item indices to mark them as completed.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		request := &todo.ShowTodoListItemsRequest{
			Title: args[0],
		}
		response, err := grpcClient.ShowTodoListItems(ctx, request)
		if err != nil {
			errors.Showerrors(err, args)
			return
		}

		if len(response.Items) == 0 {
			ui.Info(fmt.Sprintf("List '%s' is empty", args[0]))
			return
		}

		ui.Header(fmt.Sprintf("Items in \"%s\":", args[0]))
		mapping := ui.CompleteUi(response.Items, request.Title)

		if len(mapping) == 0 {
			ui.Info("All items are already completed!")
			return
		}

		fmt.Printf("\nEnter the indices of items to mark as completed (space-separated): ")
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			ui.Error(fmt.Sprintf("Error reading input: %v", err))
			return
		}
		input = strings.TrimSpace(input)

		fields := strings.Fields(input)

		var indices []int32
		for _, field := range fields {
			num, err := strconv.Atoi(field)
			if err != nil {
				ui.Error(fmt.Sprintf("Invalid index '%s'", field))
				return
			}
			if num < 1 || num > len(mapping) {
				ui.Error(fmt.Sprintf("Index %d is out of range (valid: 1-%d)", num, len(mapping)))
				return
			}
			indices = append(indices, mapping[num-1])
		}

		if len(indices) == 0 {
			ui.Info("No indices provided")
			return
		}

		deleteItemsRequest := &todo.DeleteTodoListItemsRequest{
			Title:       args[0],
			ItemIndexes: indices,
		}

		_, err = grpcClient.DeleteTodoListItems(ctx, deleteItemsRequest)
		if err != nil {
			errors.Showerrors(err, args)
			return
		}

		if len(indices) == 1 {
			ui.Success("Marked 1 item as completed")
		} else {
			ui.Success(fmt.Sprintf("Marked %d items as completed", len(indices)))
		}
	},
}

func init() {
	rootCmd.AddCommand(completeCmd)
}
