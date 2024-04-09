package capturetypes

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResetParsingError(t *testing.T) {

	errs := ParsingErrTracker{}

	errs[ErrnoInvalidIPHeader] = 1
	errs[ErrnoPacketTruncated] = 2
	errs[ErrnoPacketFragmentIgnore] = 3

	require.Equal(t, 1, errs[ErrnoInvalidIPHeader])
	require.Equal(t, 2, errs[ErrnoPacketTruncated])
	require.Equal(t, 3, errs[ErrnoPacketFragmentIgnore])
	require.Equal(t, errs.Sum(), 6)
	require.Equal(t, errs.SumFailed(), 3)

	// Create a copy of the tracker
	errCopy := errs

	// Reset the tracker
	errs.Reset()

	// Validate that the initial tracker is reset
	require.Equal(t, 0, errs[ErrnoInvalidIPHeader])
	require.Equal(t, 0, errs[ErrnoPacketTruncated])
	require.Equal(t, 0, errs[ErrnoPacketFragmentIgnore])
	require.Equal(t, errs.SumFailed(), 0)

	// Validate that the copy is not affected by the reset
	require.Equal(t, 1, errCopy[ErrnoInvalidIPHeader])
	require.Equal(t, 2, errCopy[ErrnoPacketTruncated])
	require.Equal(t, 3, errCopy[ErrnoPacketFragmentIgnore])
	require.Equal(t, errCopy.SumFailed(), 3)
}
