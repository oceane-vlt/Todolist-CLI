package storage

import "strings"

func displayList(data *TodoData) string {
	var res strings.Builder
	for title := range data.Lists {
		res.WriteString("- " + title + "\n")
	}
	return res.String()
}

func findListKey(data *TodoData, title string) string {
	titleLower := strings.ToLower(title)
	for key := range data.Lists {
		if strings.ToLower(key) == titleLower {
			return key
		}
	}
	return ""
}
