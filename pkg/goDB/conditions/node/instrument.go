/////////////////////////////////////////////////////////////////////////////////
//
// instrument.go
//
// This file contains functionality for instrumenting conditions with comparison
// functions. Each conditionNode has a field that references a comparison function
// (closure) that is specialized for carrying out the specific type of comparison
// represented by the conditionNode.
//
// Written by Lennart Elsen      lel@open.ch and
//            Lorenz Breidenbach lob@open.ch, October 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package node

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/els0r/goProbe/v4/pkg/goDB/protocols"
	"github.com/els0r/goProbe/v4/pkg/types"
)

// Returns an identical version of the receiver instrumented
// with closures (the conditionNode.compareCurrentValue) for efficient
// evaluation.
func instrument(node Node) (Node, error) {
	return node.transform(func(cn conditionNode) (Node, error) {
		err := generateCompareValue(&cn)
		return cn, err
	})
}

// Generates a closure for comparison operations based on the condition and adds
// it to the conditionNode. Both the value and the comparator of the condition can
// be "hard coded" into the closure as they are provided once in the condition
// and then never change throughout program execution. This reduces branching
// during query evaluation.
func generateCompareValue(condition *conditionNode) error {
	var (
		value     []byte
		netmask   int
		ipVersion types.IPVersion
		err       error
	)

	if value, netmask, ipVersion, err = conditionBytesAndNetmask(*condition); err != nil {
		return err
	}

	// generate the function based on which attribute was provided. For a small
	// amount of bytes, the check is performed directly in order to avoid the
	// overhead induced by a for loop
	switch condition.attribute {
	case types.SIPName:
		condition.ipVersion = ipVersion
		switch condition.comparator {
		case "=":
			condition.compareValue = func(currentValue types.Key) bool {
				return bytes.Equal(currentValue.GetSIP(), value)
			}
			return nil
		case "!=":
			condition.compareValue = func(currentValue types.Key) bool {
				return !bytes.Equal(currentValue.GetSIP(), value)
			}
			return nil
		default:
			return fmt.Errorf("comparator %q not allowed for attribute %q", condition.comparator, condition.attribute)
		}
	case types.DIPName:
		condition.ipVersion = ipVersion
		switch condition.comparator {
		case "=":
			condition.compareValue = func(currentValue types.Key) bool {
				return bytes.Equal(currentValue.GetDIP(), value)
			}
			return nil
		case "!=":
			condition.compareValue = func(currentValue types.Key) bool {
				return !bytes.Equal(currentValue.GetDIP(), value)
			}
			return nil
		default:
			return fmt.Errorf("comparator %q not allowed for attribute %q", condition.comparator, condition.attribute)
		}
	case "snet":
		condition.ipVersion = ipVersion
		// in case of matching networks, only the relevant bytes (e.g. those
		// at which the netmask is non-zero) have to be checked. This form of
		// lazy checking can be applied to both IPv4 and IPv6 networks
		var lenBytes int
		index := netmask / 8
		toShift := uint8(8 - netmask%8) // number of zeros in netmask

		// check if the netmask does not describe an RFC network class
		if toShift != 8 {
			// calculate relevant portion of netmask
			netmaskByte := uint8(0xff) << toShift
			lenBytes = index + 1

			// handle comparator operator. For IP based checks only EQUALS TO and
			// NOT EQUALS TO makes sense
			switch condition.comparator {
			case "=":
				condition.compareValue = func(currentValue types.Key) bool {
					ip := currentValue.GetSIP()
					// apply the netmask on the relevant byte in order to obtain
					// the network address
					ip[index] = ip[index] & netmaskByte

					return bytes.Equal(ip[:lenBytes], value[:lenBytes])
				}
				return nil
			case "!=":
				condition.compareValue = func(currentValue types.Key) bool {
					ip := currentValue.GetSIP()
					// apply the netmask on the relevant byte in order to obtain
					// the network address
					ip[index] = ip[index] & netmaskByte

					return !bytes.Equal(ip[:lenBytes], value[:lenBytes])
				}
				return nil
			default:
				return fmt.Errorf("comparator %q not allowed for attribute %q", condition.comparator, condition.attribute)
			}
		} else {
			// in case we have a multiple of 8, some bytes are left out of the
			// comparison
			switch condition.comparator {
			case "=":
				condition.compareValue = func(currentValue types.Key) bool {
					return bytes.Equal(currentValue.GetSIP()[:index], value[:index])
				}
				return nil
			case "!=":
				condition.compareValue = func(currentValue types.Key) bool {
					return !bytes.Equal(currentValue.GetSIP()[:index], value[:index])
				}
				return nil
			default:
				return fmt.Errorf("comparator %q not allowed for attribute %q", condition.comparator, condition.attribute)
			}
		}
	case "dnet":
		condition.ipVersion = ipVersion
		// in case of matching networks, only the relevant bytes (e.g. those
		// at which the netmask is non-zero) have to be checked. This form of
		// lazy checking can be applied to both IPv4 and IPv6 networks
		var lenBytes int
		index := netmask / 8
		toShift := uint8(8 - netmask%8) // number of zeros in netmask

		// check if the netmask does not describe an RFC network class
		if toShift != 8 {
			// calculate relevant portion of netmask
			netmaskByte := uint8(0xff) << toShift
			lenBytes = index + 1

			// handle comparator operator. For IP based checks only EQUALS TO and
			// NOT EQUALS TO makes sense
			switch condition.comparator {
			case "=":
				condition.compareValue = func(currentValue types.Key) bool {
					ip := currentValue.GetDIP()
					// apply the netmask on the relevant byte in order to obtain
					// the network address
					ip[index] = ip[index] & netmaskByte

					return bytes.Equal(ip[:lenBytes], value[:lenBytes])
				}
				return nil
			case "!=":
				condition.compareValue = func(currentValue types.Key) bool {
					ip := currentValue.GetDIP()
					// apply the netmask on the relevant byte in order to obtain
					// the network address
					ip[index] = ip[index] & netmaskByte

					return !bytes.Equal(ip[:lenBytes], value[:lenBytes])
				}
				return nil
			default:
				return fmt.Errorf("comparator %q not allowed for attribute %q", condition.comparator, condition.attribute)
			}
		} else {
			// in case we have a multiple of 8, some bytes are left out of the
			// comparison
			switch condition.comparator {
			case "=":
				condition.compareValue = func(currentValue types.Key) bool {
					return bytes.Equal(currentValue.GetDIP()[:index], value[:index])
				}
				return nil
			case "!=":
				condition.compareValue = func(currentValue types.Key) bool {
					return !bytes.Equal(currentValue.GetDIP()[:index], value[:index])
				}
				return nil
			default:
				return fmt.Errorf("comparator %q not allowed for attribute %q", condition.comparator, condition.attribute)
			}
		}
	case types.DportName:
		switch condition.comparator {
		case "=":
			condition.compareValue = func(currentValue types.Key) bool {
				return bytes.Equal(currentValue.GetDport(), value[:types.DportSizeof])
			}
			return nil
		case "!=":
			condition.compareValue = func(currentValue types.Key) bool {
				return !bytes.Equal(currentValue.GetDport(), value[:types.DportSizeof])
			}
			return nil
		case "<":
			condition.compareValue = func(currentValue types.Key) bool {
				return bytes.Compare(currentValue.GetDport(), value[:types.DportSizeof]) < 0
			}
			return nil
		case ">":
			condition.compareValue = func(currentValue types.Key) bool {
				return bytes.Compare(currentValue.GetDport(), value[:types.DportSizeof]) > 0
			}
			return nil
		case "<=":
			condition.compareValue = func(currentValue types.Key) bool {
				return bytes.Compare(currentValue.GetDport(), value[:types.DportSizeof]) <= 0
			}
			return nil
		case ">=":
			condition.compareValue = func(currentValue types.Key) bool {
				return bytes.Compare(currentValue.GetDport(), value[:types.DportSizeof]) >= 0
			}
			return nil
		default:
			return fmt.Errorf("comparator %q not allowed for attribute %q", condition.comparator, condition.attribute)
		}
	case types.ProtoName:
		switch condition.comparator {
		case "=":
			condition.compareValue = func(currentValue types.Key) bool {
				return currentValue.GetProto() == value[0]
			}
			return nil
		case "!=":
			condition.compareValue = func(currentValue types.Key) bool {
				return currentValue.GetProto() != value[0]
			}
			return nil
		case "<":
			condition.compareValue = func(currentValue types.Key) bool {
				return currentValue.GetProto() < value[0]
			}
			return nil
		case ">":
			condition.compareValue = func(currentValue types.Key) bool {
				return currentValue.GetProto() > value[0]
			}
			return nil
		case "<=":
			condition.compareValue = func(currentValue types.Key) bool {
				return currentValue.GetProto() <= value[0]
			}
			return nil
		case ">=":
			condition.compareValue = func(currentValue types.Key) bool {
				return currentValue.GetProto() >= value[0]
			}
			return nil
		default:
			return fmt.Errorf("comparator %q not allowed for attribute %q", condition.comparator, condition.attribute)
		}
	default:
		return fmt.Errorf("unknown attribute %q", condition.attribute)
	}
}

// conditionBytesAndNetmask returns the database's binary representation of the
// value of the given condition. It also validates the condition using attribute specific
// validation logic  (e.g. no IPv4 address with digits greater than 255).
//
// Input:
//
//	condition:    a conditionNode
//
// Output:
//
//	byte[]:       the value of the condition node serialized into a byte slice of the same
//	              format stored in the database.
//	int:          (Optionally) if the attribute of condition is a CIDR (e.g. 192.168.0.0/18)
//	              the length of the netmask (18 in this case)
//	error:        message whenever a condition was incorrectly specified
func conditionBytesAndNetmask(condition conditionNode) ([]byte, int, types.IPVersion, error) {

	// translate the indicated value into bytes
	var (
		err       error
		num       uint64
		isIn      bool
		netmask   int64
		isIPv4    bool
		ipVersion types.IPVersion
		condBytes []byte
	)

	attribute, comparator, value := condition.attribute, condition.comparator, condition.value

	switch comparator {
	case "=", "!=", "<", ">", "<=", ">=":
		switch attribute {
		case types.DIPName, types.SIPName:
			condBytes, isIPv4, err = types.IPStringToBytes(value)
			if err != nil {
				return nil, 0, types.IPVersionNone, fmt.Errorf("could not parse IP address: %s", value)
			}
			if isIPv4 {
				ipVersion = types.IPVersionV4
			} else {
				ipVersion = types.IPVersionV6
			}
		case "dnet", "snet":
			cidr := strings.Split(value, "/")
			if len(cidr) < 2 {
				return nil, 0, types.IPVersionNone, errors.New("could not get netmask. Use CIDR notation. Example: 192.168.1.17/25")
			}

			// parse netmask and run sanity checks
			if netmask, err = strconv.ParseInt(cidr[1], 10, 32); err != nil {
				return nil, 0, types.IPVersionNone, fmt.Errorf("failed to parse netmask %s. Use CIDR notation. Example: 192.168.1.17/25", cidr[1])
			}

			// check if the netmask is within allowed bounds
			isIPv6Address := strings.Contains(cidr[0], ":")

			if isIPv6Address {
				if netmask > 128 {
					return nil, 0, types.IPVersionNone, errors.New("incorrect netmask. Maximum possible value is 128 for IPv6 networks")
				}
				ipVersion = types.IPVersionV6
			} else {
				if netmask > 32 {
					return nil, 0, types.IPVersionNone, errors.New("incorrect netmask. Maximum possible value is 32 for IPv4 networks")
				}
				ipVersion = types.IPVersionV4
			}

			// get ip bytes and apply netmask
			if condBytes, _, err = types.IPStringToBytes(cidr[0]); err != nil {
				return nil, 0, types.IPVersionNone, fmt.Errorf("could not parse IP address: %s", value)
			}

			maskWidth := int64(4)
			if isIPv6Address {
				maskWidth = 16
			}

			// zero out unused bytes of IP
			for i := (netmask + 7) / 8; i < maskWidth; i++ {
				condBytes[i] = 0
			}
			// apply masking
			if netmask/8 < maskWidth {
				condBytes[netmask/8] &= uint8(0xFF) << uint8(8-(netmask%8))
			}

		case types.ProtoName:
			if num, err = strconv.ParseUint(value, 10, 8); err != nil {
				if num, isIn = protocols.GetIPProtoID(value); !isIn {
					return nil, 0, types.IPVersionNone, fmt.Errorf("could not parse protocol value: %w", err)
				}
			}

			condBytes = []byte{uint8(num & 0xff)}
		case types.DportName:
			if num, err = strconv.ParseUint(value, 10, 16); err != nil {
				return nil, 0, types.IPVersionNone, fmt.Errorf("could not parse dport value: %w", err)
			}

			condBytes = []byte{uint8(num >> 8), uint8(num & 0xff)}
		default:
			return nil, 0, types.IPVersionNone, fmt.Errorf("unknown attribute: %s", attribute)
		}
	default:
		return nil, 0, types.IPVersionNone, fmt.Errorf("unknown comparator: %s", comparator)
	}

	return condBytes, int(netmask), ipVersion, nil
}
