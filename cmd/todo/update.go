package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/oceane-vlt/todolist/libs/errors"
	"github.com/oceane-vlt/todolist/libs/tui"
	"github.com/oceane-vlt/todolist/libs/ui"
	todo "github.com/oceane-vlt/todolist/proto"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "update an item from a todo list",
	Long: `Update item from todo list.
	Usage:
   - Update item from the list: todo update mylist`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		// Editing is inherently interactive; say so up front rather than listing
		// the items and only then failing to open the editor.
		if !tui.IsInteractive() {
			ui.Error("'todo update' needs a terminal (it opens an interactive editor).")
			ui.Info(fmt.Sprintf("To change an item non-interactively, delete it and re-add it with %s.",
				ui.Command("todo add "+args[0]+" \"<title>\" -d \"<description>\"")))
			os.Exit(1)
		}

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

		mapping := ui.UpdateUi(response.Items, request.Title)
		fmt.Println()
		for {
			fmt.Printf("Enter the indices of the item you want to update: ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				ui.Error(fmt.Sprintf("Error reading input: %v", err))
				continue
			}
			input = strings.TrimSpace(input)

			if input == "" {
				ui.Info("No indices provided")
				return
			}

			fields := strings.Fields(input)
			if len(fields) > 1 {
				ui.Error("You can only provide 1 item to be updated")
				continue
			}
			index := fields[0]
			idx, err := strconv.Atoi(index)
			if err != nil || idx < 1 || idx > len(mapping) {
				ui.Error("Invalid index. Please enter a valid index.")
				continue
			}

			actualIndex := mapping[idx-1]
			currentItem := response.Items[actualIndex]

			// The editor is pre-filled with the current values, so this is an edit
			// rather than a re-entry, and clearing the description removes it.
			edit, err := tui.EditItem(currentItem.Title, currentItem.Description)
			if err != nil {
				ui.Error(fmt.Sprintf("Editor unavailable: %v", err))
				return
			}
			if !edit.Saved {
				ui.Info("Cancelled, nothing was changed.")
				return
			}

			// Send only what actually changed: the request fields carry presence,
			// so an untouched field is left alone server-side instead of being
			// rewritten with the value we just read back.
			updateRequest := &todo.UpdateTodoListItemRequest{
				Title:     args[0],
				ItemIndex: actualIndex,
			}
			if edit.Title != currentItem.Title {
				updateRequest.NewTitle = &edit.Title
			}
			if edit.Description != currentItem.Description {
				updateRequest.NewDescription = &edit.Description
			}

			if updateRequest.NewTitle == nil && updateRequest.NewDescription == nil {
				ui.Info("No changes.")
				return
			}

			if _, err := grpcClient.UpdateTodoListItem(ctx, updateRequest); err != nil {
				errors.Showerrors(err, args)
				return
			}
			ui.Success("Item updated successfully")

			break
		}
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
