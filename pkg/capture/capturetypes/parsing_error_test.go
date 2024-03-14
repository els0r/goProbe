package capturetypes

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResetParsingError(t *testing.T) {

	errs := ParsingErrTracker{}

	errs[ErrnoInvalidIPHeader] = 1
	errs[ErrnoPacketTruncated] = 2

	require.Equal(t, 1, errs[ErrnoInvalidIPHeader])
	require.Equal(t, 2, errs[ErrnoPacketTruncated])
	require.Equal(t, errs.Sum(), 3)

	errs.Reset()

	require.Equal(t, 0, errs[ErrnoInvalidIPHeader])
	require.Equal(t, 0, errs[ErrnoPacketTruncated])
	require.Equal(t, errs.Sum(), 0)

}
