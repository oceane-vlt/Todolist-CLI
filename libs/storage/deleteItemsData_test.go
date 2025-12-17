package storage

import (
	"reflect"
	"testing"
)

func TestDeleteItems(t *testing.T) {
	tests := []struct {
		name             string
		items            []TodoItem
		indicesToDelete  []int32
		expectedResult   []TodoItem
	}{
		{
			name: "delete single item from middle",
			items: []TodoItem{
				{Title: "Task 1", Description: "First task"},
				{Title: "Task 2", Description: "Second task"},
				{Title: "Task 3", Description: "Third task"},
			},
			indicesToDelete: []int32{1},
			expectedResult: []TodoItem{
				{Title: "Task 1", Description: "First task"},
				{Title: "Task 3", Description: "Third task"},
			},
		},
		{
			name: "delete first item",
			items: []TodoItem{
				{Title: "Task 1", Description: "First task"},
				{Title: "Task 2", Description: "Second task"},
				{Title: "Task 3", Description: "Third task"},
			},
			indicesToDelete: []int32{0},
			expectedResult: []TodoItem{
				{Title: "Task 2", Description: "Second task"},
				{Title: "Task 3", Description: "Third task"},
			},
		},
		{
			name: "delete last item",
			items: []TodoItem{
				{Title: "Task 1", Description: "First task"},
				{Title: "Task 2", Description: "Second task"},
				{Title: "Task 3", Description: "Third task"},
			},
			indicesToDelete: []int32{2},
			expectedResult: []TodoItem{
				{Title: "Task 1", Description: "First task"},
				{Title: "Task 2", Description: "Second task"},
			},
		},
		{
			name: "delete multiple items",
			items: []TodoItem{
				{Title: "Task 1", Description: "First task"},
				{Title: "Task 2", Description: "Second task"},
				{Title: "Task 3", Description: "Third task"},
				{Title: "Task 4", Description: "Fourth task"},
			},
			indicesToDelete: []int32{0, 2},
			expectedResult: []TodoItem{
				{Title: "Task 2", Description: "Second task"},
				{Title: "Task 4", Description: "Fourth task"},
			},
		},
		{
			name: "delete all items",
			items: []TodoItem{
				{Title: "Task 1", Description: "First task"},
				{Title: "Task 2", Description: "Second task"},
			},
			indicesToDelete: []int32{0, 1},
			expectedResult:  []TodoItem{},
		},
		{
			name:            "delete from empty list",
			items:           []TodoItem{},
			indicesToDelete: []int32{},
			expectedResult:  []TodoItem{},
		},
		{
			name: "delete no items (empty indices)",
			items: []TodoItem{
				{Title: "Task 1", Description: "First task"},
				{Title: "Task 2", Description: "Second task"},
			},
			indicesToDelete: []int32{},
			expectedResult: []TodoItem{
				{Title: "Task 1", Description: "First task"},
				{Title: "Task 2", Description: "Second task"},
			},
		},
		{
			name: "delete items with duplicate indices (should work)",
			items: []TodoItem{
				{Title: "Task 1", Description: "First task"},
				{Title: "Task 2", Description: "Second task"},
				{Title: "Task 3", Description: "Third task"},
			},
			indicesToDelete: []int32{1, 1},
			expectedResult: []TodoItem{
				{Title: "Task 1", Description: "First task"},
				{Title: "Task 3", Description: "Third task"},
			},
		},
		{
			name: "delete non-consecutive items",
			items: []TodoItem{
				{Title: "Task 1", Description: "First task"},
				{Title: "Task 2", Description: "Second task"},
				{Title: "Task 3", Description: "Third task"},
				{Title: "Task 4", Description: "Fourth task"},
				{Title: "Task 5", Description: "Fifth task"},
			},
			indicesToDelete: []int32{1, 3},
			expectedResult: []TodoItem{
				{Title: "Task 1", Description: "First task"},
				{Title: "Task 3", Description: "Third task"},
				{Title: "Task 5", Description: "Fifth task"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deleteItems(tt.items, tt.indicesToDelete)

			if !reflect.DeepEqual(result, tt.expectedResult) {
				t.Errorf("deleteItems() = %v, want %v", result, tt.expectedResult)
			}

			// Verify length
			if len(result) != len(tt.expectedResult) {
				t.Errorf("Result length = %d, expected %d", len(result), len(tt.expectedResult))
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		item     int32
		items    []int32
		expected bool
	}{
		{
			name:     "item exists in list",
			item:     2,
			items:    []int32{1, 2, 3, 4},
			expected: true,
		},
		{
			name:     "item does not exist in list",
			item:     5,
			items:    []int32{1, 2, 3, 4},
			expected: false,
		},
		{
			name:     "item in empty list",
			item:     1,
			items:    []int32{},
			expected: false,
		},
		{
			name:     "item is first element",
			item:     0,
			items:    []int32{0, 1, 2},
			expected: true,
		},
		{
			name:     "item is last element",
			item:     5,
			items:    []int32{1, 2, 3, 5},
			expected: true,
		},
		{
			name:     "negative item",
			item:     -1,
			items:    []int32{-1, 0, 1},
			expected: true,
		},
		{
			name:     "negative item not in list",
			item:     -5,
			items:    []int32{1, 2, 3},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.item, tt.items)
			if result != tt.expected {
				t.Errorf("contains(%d, %v) = %v, want %v", tt.item, tt.items, result, tt.expected)
			}
		})
	}
}
