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

// TestListPositionsMapsBackToTheFullList is the guard on the mapping the server
// is addressed through.
func TestListPositionsMapsBackToTheFullList(t *testing.T) {
	items := []*todo.Item{
		{Title: "done first", Completed: true},
		{Title: "pending A"},
		{Title: "done again", Completed: true},
		{Title: "pending B"},
	}
	pending, originalIndex := pendingItems(items)
	p := &listPositions{fullLen: len(items), toFull: originalIndex}

	if len(pending) != 2 {
		t.Fatalf("kept %d items, want 2", len(pending))
	}
	for position, want := range map[int]int32{0: 1, 1: 3} {
		got, err := p.full(position)
		if err != nil {
			t.Fatalf("full(%d) errored: %v", position, err)
		}
		if got != want {
			t.Errorf("full(%d) = %d, want %d", position, got, want)
		}
	}

	// Out of range is an error, never a silently wrong index.
	if _, err := p.full(2); err == nil {
		t.Error("full() past the end should error")
	}
	if _, err := p.full(-1); err == nil {
		t.Error("full() below zero should error")
	}
}

// TestListPositionsStaysInStepAfterAnAdd is the subtle one: an item added from
// the browser lands at the end of BOTH lists, and everything addressed
// afterwards must still resolve correctly.
func TestListPositionsStaysInStepAfterAnAdd(t *testing.T) {
	items := []*todo.Item{
		{Title: "done", Completed: true},
		{Title: "pending"},
	}
	_, originalIndex := pendingItems(items)
	p := &listPositions{fullLen: len(items), toFull: originalIndex}

	// Browser position 0 is full index 1.
	if got, _ := p.full(0); got != 1 {
		t.Fatalf("before the add, full(0) = %d, want 1", got)
	}

	p.appended()

	// The new row is browser position 1, and full index 2 — the length the list
	// had before the append.
	got, err := p.full(1)
	if err != nil {
		t.Fatalf("full(1) errored after the add: %v", err)
	}
	if got != 2 {
		t.Errorf("after one add, full(1) = %d, want 2", got)
	}
	// The existing mapping must not have shifted.
	if got, _ := p.full(0); got != 1 {
		t.Errorf("the add moved an existing item: full(0) = %d, want 1", got)
	}

	// A second add keeps counting.
	p.appended()
	if got, _ := p.full(2); got != 3 {
		t.Errorf("after two adds, full(2) = %d, want 3", got)
	}
}
