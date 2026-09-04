package main

import (
	"testing"

	todo "github.com/oceane-vlt/todolist/proto"
)

// TestPendingItemsMapsBackToOriginalIndices is the guard against completing the
// wrong item: the browser is shown a filtered list, so its positions only mean
// something once translated back through this mapping.
func TestPendingItemsMapsBackToOriginalIndices(t *testing.T) {
	items := []*todo.Item{
		{Title: "done first", Completed: true},
		{Title: "pending A"},
		{Title: "done again", Completed: true},
		{Title: "pending B"},
	}

	pending, originalIndex := pendingItems(items)

	if len(pending) != 2 {
		t.Fatalf("kept %d items, want the 2 pending ones", len(pending))
	}
	if pending[0].Title != "pending A" || pending[1].Title != "pending B" {
		t.Fatalf("kept the wrong items: %q, %q", pending[0].Title, pending[1].Title)
	}

	// Browser position 0 is really item 1, and position 1 is really item 3.
	want := []int32{1, 3}
	if len(originalIndex) != len(want) {
		t.Fatalf("originalIndex = %v, want %v", originalIndex, want)
	}
	for i := range want {
		if originalIndex[i] != want[i] {
			t.Errorf("browser position %d maps to %d, want %d", i, originalIndex[i], want[i])
		}
	}
}

func TestPendingItemsOnEdgeCases(t *testing.T) {
	pending, index := pendingItems(nil)
	if len(pending) != 0 || len(index) != 0 {
		t.Errorf("nil list gave %v / %v, want empty", pending, index)
	}

	allDone := []*todo.Item{{Title: "a", Completed: true}, {Title: "b", Completed: true}}
	pending, index = pendingItems(allDone)
	if len(pending) != 0 || len(index) != 0 {
		t.Errorf("fully completed list gave %v / %v, want empty", pending, index)
	}
}
