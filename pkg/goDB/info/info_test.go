package info

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHostID(t *testing.T) {
	id := GetHostID("")
	require.NotEmpty(t, id)
	require.NotContains(t, id, "\n")
	require.Len(t, id, 32)
}

func TestHostIDErrorHandling(t *testing.T) {
	id, err := hostID()
	require.Nil(t, err)
	require.NotContains(t, id, "\n")
	require.Len(t, id, 32)
}

func TestHostIDFallback(t *testing.T) {
	testPath, err := os.MkdirTemp("/tmp", "hostid")
	require.Nil(t, err)
	defer os.RemoveAll(testPath)

	id, err := fallbackHostID(testPath)
	require.Nil(t, err)
	require.NotContains(t, id, "\n")
	require.Len(t, id, 32)
}

func TestHostIDFallbackErrors(t *testing.T) {

	// Since all of these error scenarios require specific setup / teardown, take different inputs and require
	// different tests after execution we do not use table driven tests here
	t.Run("empty_dir", func(t *testing.T) {
		id, err := fallbackHostID("")
		require.EqualError(t, err, "empty DB path provided")
		require.Equal(t, id, UnknownID)
	})

	t.Run("invalid_dir", func(t *testing.T) {
		id, err := fallbackHostID("/hjgfkjagdjhkad/kjagsduasgdjasg")
		require.EqualError(t, err, "database directory does not exist: stat /hjgfkjagdjhkad/kjagsduasgdjasg: no such file or directory")
		require.ErrorIs(t, err, fs.ErrNotExist)
		require.Equal(t, id, UnknownID)
	})

	t.Run("invalid_permissions", func(t *testing.T) {
		testPath, err := os.MkdirTemp("/tmp", "hostid")
		require.Nil(t, err)
		defer os.RemoveAll(testPath)
		require.Nil(t, os.Chmod(testPath, 0400))

		id, err := fallbackHostID(testPath)
		require.EqualError(t, err, fmt.Errorf("open %s: permission denied", filepath.Join(testPath, fallbackIDFileName)).Error())
		require.ErrorIs(t, err, fs.ErrPermission)
		require.Equal(t, id, UnknownID)
	})

	t.Run("missing_write_permissions", func(t *testing.T) {
		testPath, err := os.MkdirTemp("/tmp", "hostid")
		require.Nil(t, err)
		defer os.RemoveAll(testPath)
		require.Nil(t, os.Chmod(testPath, 0500))

		id, err := fallbackHostID(testPath)
		require.EqualError(t, err, fmt.Errorf("failed to store new fallback host ID: open %s: permission denied", filepath.Join(testPath, fallbackIDFileName)).Error())
		require.ErrorIs(t, err, fs.ErrPermission)
		require.Equal(t, id, UnknownID)
	})

	t.Run("not_a_dir", func(t *testing.T) {
		testPath, err := os.CreateTemp("/tmp", "hostid")
		require.Nil(t, err)
		defer os.RemoveAll(testPath.Name())

		id, err := fallbackHostID(testPath.Name())
		require.EqualError(t, err, fmt.Errorf("path %s is not a directory", testPath.Name()).Error())
		require.Equal(t, id, UnknownID)
	})
}

func TestDBExists(t *testing.T) {
	require.EqualError(t, CheckDBExists(""), "empty DB path provided")
	require.ErrorIs(t, CheckDBExists("/hjgfkjagdjhkad/kjagsduasgdjasg"), fs.ErrNotExist)
	require.EqualError(t, CheckDBExists("/hjgfkjagdjhkad/kjagsduasgdjasg"), "database directory does not exist: stat /hjgfkjagdjhkad/kjagsduasgdjasg: no such file or directory")
}
