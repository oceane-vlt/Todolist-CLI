package main

import "fmt"

type todoElement struct{
	ID int
	List string
	Value string
}

func getElements() []todoElement{
	todoElements := []todoElement{
		{
			ID:    1,
			List:  "Work",
			Value: "Complete project documentation",
		},
		{
			ID:    2,
			List:  "Work",
			Value: "Review pull requests",
		},
		{
			ID:    3,
			List:  "Work",
			Value: "Add Campaign feature",
		},
		{
			ID:    4,
			List:  "Work",
			Value: "Continue embed on Appsec Unsused route",
		},
		{
			ID:    5,
			List:  "Work",
			Value: "Dogfood codegen campaign",
		},
		
	}

	return todoElements
}

func prettyOutput(todoElements []todoElement) string {
	output := fmt.Sprintf("%d Elements present in the todolist\n", len(todoElements))
	for _, todoElm := range todoElements{
		output += fmt.Sprintf("  [%s] - %s\n", todoElm.List, todoElm.Value)
	}
	return output
}

func main () {
	todoElements := getElements()
	formatRes := prettyOutput(todoElements)
	fmt.Println(formatRes)
}
