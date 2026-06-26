package server

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// DefaultDataFilePath returns the path of the local JSON data file
// (~/.config/todolist/data.json), ensuring its parent directory exists and that
// an initial empty data file is present. This preserves the bootstrap behaviour
// that previously lived in the package init(): it is invoked when wiring the
// local JSON Store in cmd/server.
func DefaultDataFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Create config directory if it doesn't exist.
	configDir := filepath.Join(home, ".config", "todolist")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	path := filepath.Join(configDir, "data.json")

	// Create initial data file if it doesn't exist.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		initialData := []byte(`{"lists":{}}`)
		if err := os.WriteFile(path, initialData, 0644); err != nil {
			return "", fmt.Errorf("failed to create initial data file: %w", err)
		}
		log.Printf("Created initial data file at: %s", path)
	}

	log.Printf("Using data file: %s", path)
	return path, nil
}
