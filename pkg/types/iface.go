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

// This function is used to validate the input and to jugde if interfaces should
// be displayed in output table
func ValidateIfaceArgument(ifaceArgument string) ([]string, error) {
	if IsIfaceArgumentRegExp(ifaceArgument) {
		_, err := ValidateRegExp(ifaceArgument)
		return []string{ifaceArgument}, err
	}
	ifaces := strings.Split(ifaceArgument, ifaceListDelimiter)
	for _, iface := range ifaces {
		if err := ValidateIfaceName(iface); err != nil {
			return ifaces, err
		}
	}
	return ifaces, nil
}

const regExpSeparator = "/"

func IsIfaceArgumentRegExp(iface string) bool {
	return strings.HasPrefix(iface, regExpSeparator) && strings.HasSuffix(iface, regExpSeparator) && len(iface) > 2
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

func ValidateAndExtractRegExp(regExp string) (*regexp.Regexp, error) {
	if regExp == "" {
		return nil, errors.New("interface regexp is empty")
	}
	// regexp to extract regular expression between the forward slashes
	re := regexp.MustCompile(`^/(.*?)/$`)

	// Find the match
	match := re.FindStringSubmatch(regExp)

	if len(match) > 1 {
		return regexp.Compile(match[1])
	}
	return nil, fmt.Errorf("unexpected match count on regexp %s", regExp)
}
