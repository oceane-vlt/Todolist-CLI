package errors

import (
	"fmt"
	"os"
	"strings"
)

func Showerrors(err error, args []string) {
	errMsg := err.Error()

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

	fmt.Fprintf(os.Stderr, "\033[1;31mError:\033[0m %s\n", errMsg)
}
