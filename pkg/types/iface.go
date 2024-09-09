package types

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var ifaceNameRegexp = regexp.MustCompile(`^!?[a-zA-Z0-9\.:_-]{1,15}$`)

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

func ValidateAndSeparateFilters(ifaceList string) ([]string, []string, error) {
	ifaces := strings.Split(ifaceList, ",")
	var positive, negative []string
	for _, iface := range ifaces {
		if err := ValidateIfaceName(iface); err != nil {
			return nil, nil, err
		} else {
			if iface[0] == '!' {
				negative = append(negative, iface[1:])
			} else {
				positive = append(positive, iface)
			}
		}
	}
	return positive, negative, nil
}

func ValidateRegExp(regExp string) (*regexp.Regexp, error) {
	if regExp == "" {
		return nil, errors.New("interface regexp is empty")
	}
	re, reErr := regexp.Compile(regExp)
	if reErr != nil {
		return nil, reErr
	}
	return re, nil
}
