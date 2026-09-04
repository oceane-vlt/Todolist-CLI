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
	Use:   "show <list>",
	Short: "Show the items of a todo list",
	Long: `Show the items of a todo list.

In a terminal this opens an interactive browser, which is where you work on a
list: move with the arrow keys, press → (or Enter) to read an item's
description, e to edit it, a to add one, x (or Space) to tick it, and ctrl+s to
complete what you ticked. Items carrying a description are marked with a "▸".

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
		positions := &listPositions{fullLen: len(response.Items), toFull: originalIndex}
		if !flagPlain && !flagHistory && !flagDetails && len(pending) > 0 && tui.IsInteractive() {
			result, err := tui.Browse(pending, request.Title,
				itemEditor(ctx, request.Title, positions),
				itemAdder(ctx, request.Title, positions))
			if err == nil {
				if result.Confirmed {
					completeItems(ctx, args, request.Title, result.Completed, positions)
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

// listPositions maps the browser's positions onto positions in the FULL list,
// which is how the server addresses items.
//
// It is shared (and mutable) across the browser's callbacks because adding an
// item changes the mapping: the new row lands at the end of the browser's list
// and at the end of the full list, so the two grow together. Were each callback
// given its own copy, an add followed by an edit or a completion would address
// the wrong item — silently, and destructively.
type listPositions struct {
	// fullLen is the number of items in the full list, and therefore the index
	// the next appended item will occupy.
	fullLen int
	// toFull maps a browser position to its index in the full list.
	toFull []int32
}

// full resolves a browser position to its index in the full list.
func (p *listPositions) full(position int) (int32, error) {
	if position < 0 || position >= len(p.toFull) {
		return 0, fmt.Errorf("item position %d is out of range", position)
	}
	return p.toFull[position], nil
}

// appended records that one item was added at the end of the full list, keeping
// the mapping in step. It must be called only after the server accepted the add.
func (p *listPositions) appended() {
	p.toFull = append(p.toFull, int32(p.fullLen))
	p.fullLen++
}

// itemAdder returns the callback the browser uses to append an item. The RPC is
// UpdateTodoList, which appends — which is what makes the new item's index the
// list's previous length, and what listPositions.appended relies on.
func itemAdder(ctx context.Context, listTitle string, positions *listPositions) tui.ItemAdder {
	return func(title, description string) error {
		_, err := grpcClient.UpdateTodoList(ctx, &todo.UpdateTodoListRequest{
			Title: listTitle,
			Items: []*todo.Item{{Title: title, Description: description}},
		})
		if err != nil {
			return err
		}
		positions.appended()
		return nil
	}
}

// itemEditor returns the callback the browser uses to save an edit. It is what
// keeps libs/tui free of any transport concern: the view collects the edit, this
// closure sends it.
//
// It translates the browser's position into the item's position in the full list
// (same reason as completeItems) and forwards only the fields the browser
// reports as changed, so field presence survives all the way to the server.
func itemEditor(ctx context.Context, listTitle string, positions *listPositions) tui.ItemEditor {
	return func(position int, newTitle, newDescription *string) error {
		index, err := positions.full(position)
		if err != nil {
			return err
		}
		_, err = grpcClient.UpdateTodoListItem(ctx, &todo.UpdateTodoListItemRequest{
			Title:          listTitle,
			ItemIndex:      index,
			NewTitle:       newTitle,
			NewDescription: newDescription,
		})
		return err
	}
}

// completeItems marks the browser's selection as completed, translating the
// browser's positions back into positions in the full list.
//
// Completion is soft: the items are kept and flagged, not removed, and stay
// visible under "todo show <list> -H".
func completeItems(ctx context.Context, args []string, listTitle string, selected []int, positions *listPositions) {
	if len(selected) == 0 {
		return
	}

	indexes := make([]int32, 0, len(selected))
	for _, position := range selected {
		index, err := positions.full(position)
		if err != nil {
			// Cannot happen — the browser only reports rows it was given — but
			// completing a wrong item is bad enough to be worth the guard.
			ui.Error(fmt.Sprintf("internal error: %v", err))
			return
		}
		indexes = append(indexes, index)
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
