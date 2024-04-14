package cmd

import (
	"testing"

	"github.com/els0r/goProbe/pkg/query"
	"github.com/stretchr/testify/require"
)

func TestTimestampHelp(t *testing.T) {
	require.NotPanics(t, func() {
		_ = buildTimestampHelpList(
			query.TimeFormatsDefault(),
			query.TimeFormatsCustom(),
			query.TimeFormatsRelative(),
		)
	})
}
