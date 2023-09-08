package info

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// CheckDBExists will return nil if a DB at path exists and otherwise the error encountered
func CheckDBExists(path string) error {
	if path == "" {
		return fmt.Errorf("empty DB path provided")
	}
	stat, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("database directory does not exist: %w", err)
		}

		return fmt.Errorf("failed to check DB directory: %w", err)
	}

	if !stat.IsDir() {
		return fmt.Errorf("path %s is not a directory", path)
	}

	return nil
}
