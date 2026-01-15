package errors

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

func Showerrors(err error, args []string) {
	errMsg := err.Error()

	fmt.Println()
	if strings.Contains(errMsg, "does not exist") {
		fmt.Fprintf(os.Stderr, "\033[1;31mError:\033[0m List '\033[1m%s\033[0m' not found.\n\n", args[0])
		fmt.Fprintf(os.Stderr, "Run \033[1;34mtodo list\033[0m to see all available lists.\n")
		os.Exit(1)
	}

	if strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "Unavailable") {
		fmt.Fprintf(os.Stderr, "\033[1;31mError:\033[0m Cannot connect to server.\n\n")
		fmt.Fprintf(os.Stderr, "Make sure the server is running:\n")
		fmt.Fprintf(os.Stderr, "  \033[1;34mmake dev\033[0m  (start server in background)\n")
		os.Exit(1)
	}

	if strings.Contains(errMsg, "out of range for todo list") {
		// Extract the index from error message like "item index 5 out of range for todo list personal"
		re := regexp.MustCompile(`item index (\d+) out of range`)
		matches := re.FindStringSubmatch(errMsg)
		if len(matches) > 1 {
			index := matches[1]
			fmt.Fprintf(os.Stderr, "\033[1;31mError:\033[0m Index \033[1m%s\033[0m is out of range for list '\033[1m%s\033[0m'.\n", index, args[0])
		} else {
			fmt.Fprintf(os.Stderr, "\033[1;31mError:\033[0m One or more provided indices are out of range for list '\033[1m%s\033[0m'.\n", args[0])
		}
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "\033[1;31mError:\033[0m %s\n", errMsg)
}
