package types

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var ifaceNameRegexp = regexp.MustCompile(`^[a-zA-Z0-9\.:_-]{1,15}$`)

func ValidateIfaceName(iface string) error {
	if iface == "" {
		return errors.New("interface list contains empty interface name")
	}

	if !ifaceNameRegexp.MatchString(iface) {
		return fmt.Errorf("interface name `%s` is invalid", iface)
	}

	return nil
}

func ValidateIfaceNames(ifaceList string) ([]string, error) {
	ifaces := strings.Split(ifaceList, ",")
	for _, iface := range ifaces {
		if err := ValidateIfaceName(iface); err != nil {
			return nil, err
		}
	}
	return ifaces, nil
}
