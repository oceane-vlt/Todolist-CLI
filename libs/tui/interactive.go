package tui

import "os"

// IsInteractive reports whether the CLI can take over the terminal: both stdin
// and stdout must be character devices.
//
// Stdout alone is not enough — a redirected stdin (`todo show l < file`, a CI
// runner, an editor's task pane) would leave the browser waiting on keystrokes
// that never come. Callers use this to choose between the interactive view and
// the static rendering in libs/ui, so that `todo show l | grep x` keeps working.
func IsInteractive() bool {
	return isTerminal(os.Stdin) && isTerminal(os.Stdout)
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
