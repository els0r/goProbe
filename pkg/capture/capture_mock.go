//go:build !slimcap_nomock
// +build !slimcap_nomock

package capture

import "github.com/fako1024/slimcap/capture"

// Source redefines any slimcap zero copy source interface type
type Source = capture.SourceZeroCopy
