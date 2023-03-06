package info

import (
	"fmt"
	"os"
)

// CheckDBExists will return nil if a DB at path exists and otherwise the error encountered
func CheckDBExists(path string) error {
	if path == "" {
		return fmt.Errorf("empty DB path provided")
	}
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("database directory does not exist at %s", path)
		}
		return fmt.Errorf("failed to check DB directory: %w", err)
	}
	return nil
}
