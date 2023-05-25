package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type ipVersionTest struct {
	left, right IPVersion
	excepted    IPVersion
}

func TestIPVersionMerge(t *testing.T) {
	for _, test := range []ipVersionTest{
		{IPVersionBoth, IPVersionBoth, IPVersionBoth},
		{IPVersionNone, IPVersionNone, IPVersionNone},
		{IPVersionV4, IPVersionV4, IPVersionV4},
		{IPVersionV6, IPVersionV6, IPVersionV6},
		{IPVersionNone, IPVersionBoth, IPVersionBoth},
		{IPVersionBoth, IPVersionNone, IPVersionBoth},
		{IPVersionV4, IPVersionV6, IPVersionBoth},
		{IPVersionV6, IPVersionV4, IPVersionBoth},
		{IPVersionBoth, IPVersionV4, IPVersionBoth},
		{IPVersionBoth, IPVersionV6, IPVersionBoth},
		{IPVersionV4, IPVersionBoth, IPVersionBoth},
		{IPVersionV6, IPVersionBoth, IPVersionBoth},
		{IPVersionNone, IPVersionV4, IPVersionV4},
		{IPVersionNone, IPVersionV6, IPVersionV6},
		{IPVersionV4, IPVersionNone, IPVersionV4},
		{IPVersionV6, IPVersionNone, IPVersionV6},
	} {
		require.Equalf(t, test.excepted, test.left.Merge(test.right), "left: %d, right %d", test.left, test.right)
	}
}
