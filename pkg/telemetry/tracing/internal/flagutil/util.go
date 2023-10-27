package flagutil

import (
	"fmt"
	"strings"
)

// WithSupported will append the supported values to the help text
func WithSupported(help string, supported []string) string {
	return fmt.Sprintf("%s (%s)", help, strings.Join(supported, ", "))
}
