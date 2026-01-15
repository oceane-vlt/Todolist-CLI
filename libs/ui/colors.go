package ui

import (
	"fmt"
	"os"
)

// Color codes
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorGray   = "\033[90m"
	Bold        = "\033[1m"
	BoldRed     = "\033[1;31m"
	BoldGreen   = "\033[1;32m"
	BoldBlue    = "\033[1;34m"
)

// Success prints a success message in green with checkmark
func Success(msg string) {
	fmt.Printf("%s✓%s %s\n", BoldGreen, ColorReset, msg)
}

// Error prints an error message in red to stderr
func Error(msg string) {
	fmt.Fprintf(os.Stderr, "%sError:%s %s\n", BoldRed, ColorReset, msg)
}

// Info prints an info message
func Info(msg string) {
	fmt.Println(msg)
}

// Command formats a command in blue and bold
func Command(cmd string) string {
	return fmt.Sprintf("%s%s%s", BoldBlue, cmd, ColorReset)
}

// Bold formats text in bold
func BoldText(text string) string {
	return fmt.Sprintf("%s%s%s", Bold, text, ColorReset)
}

// ListItem prints a todo item with checkbox
func ListItem(index int, title string, completed bool) {
	checkbox := "[ ]"
	color := ColorReset
	if completed {
		checkbox = "[✓]"
		color = ColorGray
	}
	fmt.Printf("  %d. %s%s %s%s\n", index+1, color, checkbox, title, ColorReset)
}

// Header prints a section header
func Header(title string) {
	fmt.Printf("\n%s%s%s\n\n", Bold, title, ColorReset)
}
