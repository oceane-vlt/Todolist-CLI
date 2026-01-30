package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/oceane-vlt/todolist/libs/errors"
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

			fmt.Printf("\nEditing item: %s\n", currentItem.Title)

			prompt := promptui.Prompt{
				Label:   "New title",
				Default: currentItem.Title,
			}

			newTitle, err := prompt.Run()
			if err != nil {
				ui.Error(fmt.Sprintf("Error reading input: %v", err))
				continue
			}

			newTitle = strings.TrimSpace(newTitle)

			updateRequest := &todo.UpdateTodoListItemRequest{
				Title:     args[0],
				ItemIndex: actualIndex,
				NewTitle:  newTitle,
			}

			_, error := grpcClient.UpdateTodoListItem(ctx, updateRequest)
			if error != nil {
				errors.Showerrors(error, args)
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
