package main

import (
	"context"
	"fmt"
	"os"

	"github.com/oceane-vlt/todolist/libs/errors"
	"github.com/oceane-vlt/todolist/libs/tui"
	"github.com/oceane-vlt/todolist/libs/ui"
	todo "github.com/oceane-vlt/todolist/proto"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the items of a todo list",
	Long: `Show the items of a todo list.

In a terminal this opens an interactive browser: move with the arrow keys and
press → (or Enter) on an item to read its description. Items carrying a
description are marked with a "▸".

When the output is piped or redirected the command prints a static list instead,
so it stays usable in scripts.

Usage:
  - Browse a list interactively:  todo show mylist
  - Force the static rendering:   todo show mylist --plain
  - Print descriptions inline:    todo show mylist --details
  - Show full history (all items): todo show mylist -H`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		flagHistory, _ := cmd.Flags().GetBool("history")
		flagPlain, _ := cmd.Flags().GetBool("plain")
		flagDetails, _ := cmd.Flags().GetBool("details")

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

		// The interactive browser is the default in a terminal, but never the
		// only way out: -H, --details and --plain all ask for the static
		// rendering, and a non-terminal stdout falls back to it as well.
		//
		// A list whose items are all completed has nothing to browse, so it goes
		// to the static rendering too — which is what prints the "all completed"
		// message and the history.
		pending, originalIndex := pendingItems(response.Items)
		if !flagPlain && !flagHistory && !flagDetails && len(pending) > 0 && tui.IsInteractive() {
			result, err := tui.Browse(pending, request.Title)
			if err == nil {
				if result.Confirmed {
					completeItems(ctx, args, request.Title, result.Completed, originalIndex)
				}
				return
			}
			// Falling back rather than failing: the user asked to see their list,
			// and the static rendering still answers that. The reason is reported
			// so the failure is not silent.
			ui.Error(fmt.Sprintf("interactive view unavailable (%v), falling back to plain output", err))
		}

		ui.ShowUi(response.Items, request.Title, flagHistory, flagDetails)
		fmt.Println()
	},
}

// pendingItems keeps the not-completed items, which are what the interactive
// browser shows: it is a working view, and the completed history has its own
// flag (-H) on the static rendering.
//
// It also returns, for each kept item, its index in the ORIGINAL list. The
// server addresses items by their position in the full list, so completing what
// the browser reports without this mapping would complete the wrong items as
// soon as the list contains a single completed one.
func pendingItems(items []*todo.Item) (pending []*todo.Item, originalIndex []int32) {
	pending = make([]*todo.Item, 0, len(items))
	originalIndex = make([]int32, 0, len(items))
	for i, item := range items {
		if !item.Completed {
			pending = append(pending, item)
			originalIndex = append(originalIndex, int32(i))
		}
	}
	return pending, originalIndex
}

// completeItems marks the browser's selection as completed, translating the
// browser's positions back into positions in the full list.
//
// Completion is soft: the items are kept and flagged, not removed, and stay
// visible under "todo show <list> -H".
func completeItems(ctx context.Context, args []string, listTitle string, selected []int, originalIndex []int32) {
	if len(selected) == 0 {
		return
	}

	indexes := make([]int32, 0, len(selected))
	for _, position := range selected {
		if position < 0 || position >= len(originalIndex) {
			// Cannot happen — the browser only reports rows it was given — but
			// completing a wrong item is bad enough to be worth the guard.
			ui.Error(fmt.Sprintf("internal error: item position %d is out of range", position))
			return
		}
		indexes = append(indexes, originalIndex[position])
	}

	if _, err := grpcClient.DeleteTodoListItems(ctx, &todo.DeleteTodoListItemsRequest{
		Title:       listTitle,
		ItemIndexes: indexes,
	}); err != nil {
		errors.Showerrors(err, args)
		return
	}

	if len(indexes) == 1 {
		ui.Success("Completed 1 item.")
	} else {
		ui.Success(fmt.Sprintf("Completed %d items.", len(indexes)))
	}
	ui.Info("Completed items are kept — see them with " + ui.Command("todo show "+listTitle+" -H") + ".")
}

func init() {
	showCmd.Flags().BoolP("verbose", "v", false, "enable verbose output")
	showCmd.Flags().BoolP("history", "H", false, "Show full completed items history")
	showCmd.Flags().Bool("plain", false, "Force the static (non-interactive) rendering")
	showCmd.Flags().BoolP("details", "d", false, "Print item descriptions inline (implies --plain)")
	rootCmd.AddCommand(showCmd)
}
