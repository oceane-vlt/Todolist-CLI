package storage

import (
	"testing"
)

func TestParseTodoListNames(t *testing.T) {
	tests := []struct {
		name            string
		jsonContent     string
		expectedLists   map[string]int32 // title -> size
		expectError     bool
		checkExactOrder bool
	}{
		{
			name: "valid json with multiple lists",
			jsonContent: `{
				"lists": {
					"work": [
						{
							"id": 1,
							"title": "Learn JavaScript",
							"description": "Complete the JavaScript fundamentals course",
							"completed": true,
							"dueDate": "2024-01-15",
							"priority": "high"
						}
					],
					"personal": [
						{
							"id": 1,
							"title": "Grocery shopping",
							"description": "Buy ingredients for the week",
							"completed": true,
							"dueDate": "2024-01-10",
							"priority": "medium"
						}
					],
					"learning": [
						{
							"id": 1,
							"title": "Read Go book",
							"description": "Finish reading The Go Programming Language",
							"completed": false,
							"dueDate": "",
							"priority": "high"
						}
					]
				}
			}`,
			expectedLists: map[string]int32{
				"work":     1,
				"personal": 1,
				"learning": 1,
			},
			expectError:     false,
			checkExactOrder: false,
		},
		{
			name: "empty lists object",
			jsonContent: `{
				"lists": {}
			}`,
			expectedLists:   map[string]int32{},
			expectError:     false,
			checkExactOrder: true,
		},
		{
			name: "single list with multiple items",
			jsonContent: `{
				"lists": {
					"work": [
						{
							"id": 1,
							"title": "Task 1",
							"description": "Description",
							"completed": false,
							"dueDate": "2024-01-15",
							"priority": "high"
						},
						{
							"id": 2,
							"title": "Task 2",
							"description": "Another task",
							"completed": false,
							"dueDate": "2024-01-16",
							"priority": "low"
						}
					]
				}
			}`,
			expectedLists: map[string]int32{
				"work": 2,
			},
			expectError:     false,
			checkExactOrder: true,
		},
		{
			name: "multiple list categories with empty lists",
			jsonContent: `{
				"lists": {
					"urgent": [],
					"backlog": [],
					"completed": []
				}
			}`,
			expectedLists: map[string]int32{
				"urgent":    0,
				"backlog":   0,
				"completed": 0,
			},
			expectError:     false,
			checkExactOrder: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the parsing function directly with JSON data
			result := parseTodoListNames([]byte(tt.jsonContent))

			// Verify results
			if tt.expectError {
				if len(result) > 0 {
					t.Errorf("Expected error or nil, but got list: %v", result)
				}
				return
			}

			if result == nil {
				if len(tt.expectedLists) == 0 {
					// nil is acceptable for empty expected lists
					return
				}
				t.Errorf("Expected lists %v, but got nil", tt.expectedLists)
				return
			}

			// Check length
			if len(result) != len(tt.expectedLists) {
				t.Errorf("Expected %d lists, got %d", len(tt.expectedLists), len(result))
				return
			}

			// Build a map from result for easy comparison
			resultMap := make(map[string]int32)
			for _, listSize := range result {
				resultMap[listSize.Title] = listSize.Size
			}

			// Verify each expected list
			for expectedTitle, expectedSize := range tt.expectedLists {
				actualSize, found := resultMap[expectedTitle]
				if !found {
					t.Errorf("Expected list '%s' not found in result", expectedTitle)
					continue
				}
				if actualSize != expectedSize {
					t.Errorf("List '%s': expected size %d, got %d", expectedTitle, expectedSize, actualSize)
				}
			}
		})
	}
}
