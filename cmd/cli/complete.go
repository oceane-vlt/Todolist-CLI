package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

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
			log.Fatalf("Error calling ShowTodoListItems: %v", err)
		}

		fmt.Printf("Items in todo list: %v\n", request.Title)
		for i, item := range response.Items {
			fmt.Printf("%d %s (Description: %s, Completed: %v, DueDate: %s, Priority: %s)\n",
				i, item.Title, item.Description, item.Completed, item.DueDate, item.Priority)
		}

		fmt.Println("\nEnter the indices of items to mark as completed (space-separated):")
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			log.Fatalf("Error reading input: %v", err)
		}
		input = strings.TrimSpace(input)

		fields := strings.Fields(input)

		var indices []int32
		for _, field := range fields {
			num, err := strconv.Atoi(field)
			if err != nil {
				log.Fatalf("Error parsing index '%s': %v", field, err)
			}
			indices = append(indices, int32(num))
		}

		if len(indices) == 0 {
			fmt.Println("No indices provided")
			return
		}

		// fmt.Printf("Indices to mark as completed: %v\n", indices)
		deleteItemsRequest := &todo.DeleteTodoListItemsRequest{
			Title:       args[0],
			ItemIndexes: indices,
		}

		_, err = grpcClient.DeleteTodoListItems(ctx, deleteItemsRequest)
		if err != nil {
			log.Fatalf("Error calling DeleteTodoListItems: %v", err)
		}

		fmt.Println("Selected items marked as completed successfully.")
	},
}

func init() {
	rootCmd.AddCommand(completeCmd)
}
