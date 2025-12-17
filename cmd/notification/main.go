package main

import (
	"fmt"
)

type todoElement struct {
	ID    int
	List  string
	Value string
}

func getElements() []todoElement {
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
		{
			ID:    6,
			List:  "Personal",
			Value: "Buy groceries for the week",
		},
		{
			ID:    7,
			List:  "Personal",
			Value: "Schedule dentist appointment",
		},
		{
			ID:    8,
			List:  "Personal",
			Value: "Call mom on her birthday",
		},
		{
			ID:    9,
			List:  "Learning",
			Value: "Complete Go tutorial on concurrency",
		},
		{
			ID:    10,
			List:  "Learning",
			Value: "Read chapter 5 of Clean Code",
		},
	}

	return todoElements
}

func outputMap(catagory string, todoElements []todoElement) string {
	output := fmt.Sprintf("You have %d elements in the %s catagory\n", len(todoElements), catagory)

	for _, todoElm := range todoElements {
		output += fmt.Sprintf(" - %s\n", todoElm.Value)
	}

	return output

}
func prettyOutput(todoElements []todoElement) string {
	output := "Your Amazing Todolist\n"
	elemByCat := make(map[string][]todoElement)
	for _, todoElm := range todoElements {
		elemByCat[todoElm.List] = append(elemByCat[todoElm.List], todoElm)
	}
	for c, elms := range elemByCat {
		output += outputMap(c, elms)
	}
	return output
}

func main() {
	todoElements := getElements()
	formatRes := prettyOutput(todoElements)
	fmt.Println(formatRes)
}
