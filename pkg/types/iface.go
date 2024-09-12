package types

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var ifaceNameRegexp = regexp.MustCompile(`^!?[a-zA-Z0-9\.:_-]{1,15}$`)

const ifaceListDelimiter = ","

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
	ifaces := strings.Split(ifaceList, ifaceListDelimiter)
	for _, iface := range ifaces {
		if err := ValidateIfaceName(iface); err != nil {
			return nil, err
		}
	}
	return ifaces, nil
}

func ValidateAndSeparateFilters(ifaceList string) ([]string, []string, error) {
	ifaces := strings.Split(ifaceList, ifaceListDelimiter)
	var positive, negative []string
	for _, iface := range ifaces {
		if err := ValidateIfaceName(iface); err != nil {
			return nil, nil, err
		}
		if strings.HasPrefix(iface, "!") {
			negative = append(negative, iface[1:])
		} else {
			positive = append(positive, iface)
		}
	}
	return positive, negative, nil
}

func ValidateRegExp(regExp string) (*regexp.Regexp, error) {
	if regExp == "" {
		return nil, errors.New("interface regexp is empty")
	}
	return regexp.Compile(regExp)
}
