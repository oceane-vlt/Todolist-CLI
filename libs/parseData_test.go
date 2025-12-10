package libs

import (
	"reflect"
	"testing"
)

func TestParseTodoListNames(t *testing.T) {
	tests := []struct {
		name           string
		jsonContent    string
		expectedLists  []string
		expectError    bool
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
			expectedLists: []string{"work", "personal", "learning"},
			expectError:    false,
			checkExactOrder: false, // Map iteration order is not guaranteed
		},
		{
			name: "empty lists object",
			jsonContent: `{
				"lists": {}
			}`,
			expectedLists: []string{},
			expectError:    false,
			checkExactOrder: true,
		},
		{
			name: "single list",
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
						}
					]
				}
			}`,
			expectedLists: []string{"work"},
			expectError:    false,
			checkExactOrder: true,
		},
		{
			name: "multiple list categories",
			jsonContent: `{
				"lists": {
					"urgent": [],
					"backlog": [],
					"completed": []
				}
			}`,
			expectedLists: []string{"urgent", "backlog", "completed"},
			expectError:    false,
			checkExactOrder: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the parsing function directly with JSON data
			titles := parseTodoListNames([]byte(tt.jsonContent))

			// Verify results
			if tt.expectError {
				if titles != nil && len(titles) > 0 {
					t.Errorf("Expected error or nil, but got list names: %v", titles)
				}
				return
			}

			if titles == nil {
				if len(tt.expectedLists) == 0 {
					// nil is acceptable for empty expected lists
					return
				}
				t.Errorf("Expected list names %v, but got nil", tt.expectedLists)
				return
			}

			// Check length
			if len(titles) != len(tt.expectedLists) {
				t.Errorf("Expected %d list names, got %d. Expected: %v, Got: %v",
					len(tt.expectedLists), len(titles), tt.expectedLists, titles)
				return
			}

			// If exact order matters, use DeepEqual
			if tt.checkExactOrder {
				if !reflect.DeepEqual(titles, tt.expectedLists) {
					t.Errorf("Expected list names %v, got %v", tt.expectedLists, titles)
				}
			} else {
				// If order doesn't matter, check that all expected lists are present
				titleMap := make(map[string]bool)
				for _, title := range titles {
					titleMap[title] = true
				}

				for _, expected := range tt.expectedLists {
					if !titleMap[expected] {
						t.Errorf("Expected list name '%s' not found in result: %v", expected, titles)
					}
				}
			}
		})
	}
}
