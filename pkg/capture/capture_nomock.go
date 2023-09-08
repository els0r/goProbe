//go:build slimcap_nomock
// +build slimcap_nomock

package capture

import "github.com/fako1024/slimcap/capture/afpacket/afring"

// Source redefines an afring source
type Source = *afring.Source
