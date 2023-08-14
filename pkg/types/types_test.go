package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type ipVersionTest struct {
	left, right IPVersion
	expected    IPVersion
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
		require.Equalf(t, test.expected, test.left.Merge(test.right), "left: %d, right %d", test.left, test.right)
	}
}

type ipAddrParseTest struct {
	input          string
	expectedData   []byte
	expectedIsIPv4 bool
	expectedErr    error
}

func TestIPAddressStringParsing(t *testing.T) {
	for _, test := range []ipAddrParseTest{
		{"", nil, false, ErrIncorrectIPAddrFormat},
		{"1.2.3.400", nil, false, ErrIncorrectIPAddrFormat},
		{"1.2.3.4", []byte{1, 2, 3, 4}, true, nil},
		{"8beb:5b74:6897:209c:ad4b:36d8:8825:8d8666", nil, false, ErrIncorrectIPAddrFormat},
		{"8beb:5b74:6897:209c:ad4b:36d8:8825:8d86", []byte{139, 235, 91, 116, 104, 151, 32, 156, 173, 75, 54, 216, 136, 37, 141, 134}, false, nil},
	} {
		ipData, isIPv4, err := IPStringToBytes(test.input)

		require.Equalf(t, test.expectedData, ipData, "want: %v , have %v", test.expectedData, ipData)
		require.Equal(t, test.expectedIsIPv4, isIPv4)
		require.Equal(t, test.expectedErr, err)
	}
}
