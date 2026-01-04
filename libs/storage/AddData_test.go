package storage

import (
	"reflect"
	"testing"

	todo "github.com/oceane-vlt/todolist/proto"
)

func TestUpdateData(t *testing.T) {
	tests := []struct {
		name           string
		listToUpdate   []TodoItem
		itemsToAdd     []*todo.Item
		expectedResult []TodoItem
		expectError    bool
	}{
		{
			name: "add single item",
			listToUpdate: []TodoItem{
				{Title: "Task 1", Description: "First task"},
				{Title: "Task 2", Description: "Second task"},
				{Title: "Task 3", Description: "Third task"},
			},
			itemsToAdd: []*todo.Item{
				{Title: "Task 4"},
			},
			expectedResult: []TodoItem{
				{Title: "Task 1", Description: "First task"},
				{Title: "Task 2", Description: "Second task"},
				{Title: "Task 3", Description: "Third task"},
				{Title: "Task 4"},
			},
			expectError: false,
		},
		{
			name: "add multiple items",
			listToUpdate: []TodoItem{
				{Title: "Task 1", Description: "First task"},
				{Title: "Task 2", Description: "Second task"},
				{Title: "Task 3", Description: "Third task"},
			},
			itemsToAdd: []*todo.Item{
				{Title: "Task 4"},
				{Title: "Task 5"},
			},
			expectedResult: []TodoItem{
				{Title: "Task 1", Description: "First task"},
				{Title: "Task 2", Description: "Second task"},
				{Title: "Task 3", Description: "Third task"},
				{Title: "Task 4"},
				{Title: "Task 5"},
			},
			expectError: false,
		},
		{
			name:         "add items to empty list",
			listToUpdate: []TodoItem{},
			itemsToAdd: []*todo.Item{
				{Title: "First task"},
				{Title: "Second task"},
			},
			expectedResult: []TodoItem{
				{Title: "First task"},
				{Title: "Second task"},
			},
			expectError: false,
		},
		{
			name: "add item with all fields",
			listToUpdate: []TodoItem{
				{Title: "Existing task"},
			},
			itemsToAdd: []*todo.Item{
				{
					Title:       "New task",
					Description: "Detailed description",
					Completed:   true,
					DueDate:     "2024-12-31",
					Priority:    "high",
				},
			},
			expectedResult: []TodoItem{
				{Title: "Existing task"},
				{Title: "New task"},
			},
			expectError: false,
		},
		{
			name: "add no items",
			listToUpdate: []TodoItem{
				{Title: "Task 1"},
				{Title: "Task 2"},
			},
			itemsToAdd: []*todo.Item{},
			expectedResult: []TodoItem{
				{Title: "Task 1"},
				{Title: "Task 2"},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := updateData(tt.listToUpdate, tt.itemsToAdd)

			// Check error expectation
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Check result
			if !reflect.DeepEqual(result, tt.expectedResult) {
				t.Errorf("updateData() = %v, want %v", result, tt.expectedResult)
			}

			// Verify length
			if len(result) != len(tt.expectedResult) {
				t.Errorf("Result length = %d, expected %d", len(result), len(tt.expectedResult))
			}
		})
	}
}
