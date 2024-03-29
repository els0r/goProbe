package info

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (

	// UnknownID denotes an unknown / non-determinable host ID
	UnknownID = "UNKNOWN"

	fallbackIDFileName = "host.id"
	hostIDLen          = 16 // 128 bit in line with defaults of /etc/machine-id
)

var runtimeID string

func init() {
	runtimeID = generateRuntimeID()
}

// GetHostID is a method that returns a system's unique identifier
func GetHostID(fallbackPath string) string {
	id, err := hostID()
	if err != nil {

		// In case a fallback location was provided, attempt to read from or
		// generate a new host ID there
		if fallbackPath != "" {
			id, _ = fallbackHostID(fallbackPath)
		}
	}

	return id
}

// RuntimeID returns the unique runtime ID of this binary
func RuntimeID() string {
	return runtimeID
}

func fallbackHostID(basePath string) (string, error) {

	// Ascertain that the provided directory exists
	if err := CheckDBExists(basePath); err != nil {
		return UnknownID, err
	}

	// Attempt to read the fallback ID
	fallbackIDPath := filepath.Join(basePath, fallbackIDFileName)
	idData, err := os.ReadFile(filepath.Clean(fallbackIDPath))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {

			// Fallback ID does not yet exist, generate it
			newID, err := generateHostID()
			if err != nil {
				return UnknownID, fmt.Errorf("failed to generate new fallback host ID: %w", err)
			}
			if err = os.WriteFile(fallbackIDPath, []byte(newID), 0600); err != nil {
				return UnknownID, fmt.Errorf("failed to store new fallback host ID: %w", err)
			}

			return newID, nil
		}

		return UnknownID, err
	}

	return sanitizeHostID(idData), nil
}

func generateHostID() (string, error) {
	b := make([]byte, hostIDLen)
	n, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	if n != hostIDLen {
		return "", fmt.Errorf("incorrect number of random bytes, want %d, have %d", hostIDLen, n)
	}

	return fmt.Sprintf("%x", b), nil
}

func sanitizeHostID(idData []byte) string {
	return strings.TrimRight(string(idData), "\n")
}

func generateRuntimeID() string {
	hash := sha256.Sum256([]byte(time.Now().UTC().String()))
	return fmt.Sprintf("%s_%x", GetHostID(""), hash)
}
