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
	Italic      = "\033[3m"
	BoldRed     = "\033[1;31m"
	BoldGreen   = "\033[1;32m"
	BoldBlue    = "\033[1;34m"

	// Dim (SGR 2, "faint") is what pushes chrome — checkboxes, counters, key
	// hints — behind the content. It is preferred over a fixed grey because it
	// derives from the terminal's own foreground colour, so it stays legible in
	// both light and dark themes, where a hard-coded grey does not.
	Dim = "\033[2m"

	// Accent is the single colour used to mark the focused row. Keeping the
	// selection to one colour (rather than one per element) is what gives the
	// interactive views a readable hierarchy: accent = "you are here", dim =
	// chrome, default = content.
	Accent     = "\033[36m"
	BoldAccent = "\033[1;36m"
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

// ItalicText formats text in italic
func ItalicText(text string) string {
	return fmt.Sprintf("%s%s%s", Italic, text, ColorReset)
}

// Header prints a section header
func Header(title string) {
	fmt.Printf("\n%s%s%s\n\n", Bold, title, ColorReset)
}
