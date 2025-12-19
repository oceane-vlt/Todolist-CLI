package storage

import "strings"

func displayList(data *TodoData) string {
	var res strings.Builder
	for title := range data.Lists{
		res .WriteString("- " + title + "\n")
	}
	return res.String()
}
