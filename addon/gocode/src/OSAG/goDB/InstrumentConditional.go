/////////////////////////////////////////////////////////////////////////////////
//
// InstrumentConditional.go
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

package goDB

import (
	"bytes"
	"errors"
	"net"
	"strconv"
	"strings"
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
		value   []byte
		netmask int
		err     error
	)

	if value, netmask, err = conditionBytesAndNetmask(*condition); err != nil {
		return err
	}

	// generate the function based on which attribute was provided. For a small
	// amount of bytes, the check is performed directly in order to avoid the
	// overhead induced by a for loop
	switch condition.attribute {
	case "sip":
		switch condition.comparator {
		case "=":
			condition.compareValue = func(currentValue *ExtraKey) bool {
				return bytes.Compare(currentValue.Sip[:], value[:SIP_SIZEOF]) == 0
			}
			return nil
		case "!=":
			condition.compareValue = func(currentValue *ExtraKey) bool {
				return bytes.Compare(currentValue.Sip[:], value[:SIP_SIZEOF]) != 0
			}
			return nil
		default:
			return errors.New("Comparator \"" + condition.comparator + "\" not allowed for attribute \"" + condition.attribute + "\"")
		}
	case "dip":
		switch condition.comparator {
		case "=":
			condition.compareValue = func(currentValue *ExtraKey) bool {
				return bytes.Compare(currentValue.Dip[:], value[:DIP_SIZEOF]) == 0
			}
			return nil
		case "!=":
			condition.compareValue = func(currentValue *ExtraKey) bool {
				return bytes.Compare(currentValue.Dip[:], value[:DIP_SIZEOF]) != 0
			}
			return nil
		default:
			return errors.New("Comparator \"" + condition.comparator + "\" not allowed for attribute \"" + condition.attribute + "\"")
		}
	case "snet":
		// in case of matching networks, only the relevant bytes (e.g. those
		// at which the netmask is non-zero) have to be checked. This form of
		// lazy checking can be applied to both IPv4 and IPv6 networks
		var len_bytes int
		index := int(netmask / 8)
		toShift := uint8(8 - netmask%8) // number of zeros in netmask

		// check if the netmask does not describe an RFC network class
		if toShift != 8 {
			// calculate relevant portion of netmask
			netmaskByte := uint8(0xff) << toShift
			len_bytes = index + 1

			// handle comparator operator. For IP based checks only EQUALS TO and
			// NOT EQUALS TO makes sense
			switch condition.comparator {
			case "=":
				condition.compareValue = func(currentValue *ExtraKey) bool {
					ip := currentValue.Sip
					// apply the netmask on the relevant byte in order to obtain
					// the network address
					ip[index] = ip[index] & netmaskByte

					return bytes.Compare(ip[:len_bytes], value[:len_bytes]) == 0
				}
				return nil
			case "!=":
				condition.compareValue = func(currentValue *ExtraKey) bool {
					ip := currentValue.Sip
					// apply the netmask on the relevant byte in order to obtain
					// the network address
					ip[index] = ip[index] & netmaskByte

					return !(bytes.Compare(ip[:len_bytes], value[:len_bytes]) == 0)
				}
				return nil
			default:
				return errors.New("Comparator \"" + condition.comparator + "\" not allowed for attribute \"" + condition.attribute + "\"")
			}
		} else {
			// in case we have a multiple of 8, some bytes are left out of the
			// comparison
			switch condition.comparator {
			case "=":
				condition.compareValue = func(currentValue *ExtraKey) bool {
					return bytes.Compare(currentValue.Sip[:index], value[:index]) == 0
				}
				return nil
			case "!=":
				condition.compareValue = func(currentValue *ExtraKey) bool {
					return bytes.Compare(currentValue.Sip[:index], value[:index]) != 0
				}
				return nil
			default:
				return errors.New("Comparator \"" + condition.comparator + "\" not allowed for attribute \"" + condition.attribute + "\"")
			}
		}
	case "dnet":
		// in case of matching networks, only the relevant bytes (e.g. those
		// at which the netmask is non-zero) have to be checked. This form of
		// lazy checking can be applied to both IPv4 and IPv6 networks
		var len_bytes int
		index := int(netmask / 8)
		toShift := uint8(8 - netmask%8) // number of zeros in netmask

		// check if the netmask does not describe an RFC network class
		if toShift != 8 {
			// calculate relevant portion of netmask
			netmaskByte := uint8(0xff) << toShift
			len_bytes = index + 1

			// handle comparator operator. For IP based checks only EQUALS TO and
			// NOT EQUALS TO makes sense
			switch condition.comparator {
			case "=":
				condition.compareValue = func(currentValue *ExtraKey) bool {
					ip := currentValue.Dip
					// apply the netmask on the relevant byte in order to obtain
					// the network address
					ip[index] = ip[index] & netmaskByte

					return bytes.Compare(ip[:len_bytes], value[:len_bytes]) == 0
				}
				return nil
			case "!=":
				condition.compareValue = func(currentValue *ExtraKey) bool {
					ip := currentValue.Dip
					// apply the netmask on the relevant byte in order to obtain
					// the network address
					ip[index] = ip[index] & netmaskByte

					return !(bytes.Compare(ip[:len_bytes], value[:len_bytes]) == 0)
				}
				return nil
			default:
				return errors.New("Comparator \"" + condition.comparator + "\" not allowed for attribute \"" + condition.attribute + "\"")
			}
		} else {
			// in case we have a multiple of 8, some bytes are left out of the
			// comparison
			switch condition.comparator {
			case "=":
				condition.compareValue = func(currentValue *ExtraKey) bool {
					return bytes.Compare(currentValue.Dip[:index], value[:index]) == 0
				}
				return nil
			case "!=":
				condition.compareValue = func(currentValue *ExtraKey) bool {
					return bytes.Compare(currentValue.Dip[:index], value[:index]) != 0
				}
				return nil
			default:
				return errors.New("Comparator \"" + condition.comparator + "\" not allowed for attribute \"" + condition.attribute + "\"")
			}
		}
	case "dport":
		switch condition.comparator {
		case "=":
			condition.compareValue = func(currentValue *ExtraKey) bool {
				return bytes.Compare(currentValue.Dport[:], value[:DPORT_SIZEOF]) == 0
			}
			return nil
		case "!=":
			condition.compareValue = func(currentValue *ExtraKey) bool {
				return bytes.Compare(currentValue.Dport[:], value[:DPORT_SIZEOF]) != 0
			}
			return nil
		case "<":
			condition.compareValue = func(currentValue *ExtraKey) bool {
				return bytes.Compare(currentValue.Dport[:], value[:DPORT_SIZEOF]) < 0
			}
			return nil
		case ">":
			condition.compareValue = func(currentValue *ExtraKey) bool {
				return bytes.Compare(currentValue.Dport[:], value[:DPORT_SIZEOF]) > 0
			}
			return nil
		case "<=":
			condition.compareValue = func(currentValue *ExtraKey) bool {
				return bytes.Compare(currentValue.Dport[:], value[:DPORT_SIZEOF]) <= 0
			}
			return nil
		case ">=":
			condition.compareValue = func(currentValue *ExtraKey) bool {
				return bytes.Compare(currentValue.Dport[:], value[:DPORT_SIZEOF]) >= 0
			}
			return nil
		default:
			return errors.New("Comparator \"" + condition.comparator + "\" not allowed for attribute \"" + condition.attribute + "\"")
		}
	case "proto":
		switch condition.comparator {
		case "=":
			condition.compareValue = func(currentValue *ExtraKey) bool {
				return currentValue.Protocol == value[0]
			}
			return nil
		case "!=":
			condition.compareValue = func(currentValue *ExtraKey) bool {
				return currentValue.Protocol != value[0]
			}
			return nil
		case "<":
			condition.compareValue = func(currentValue *ExtraKey) bool {
				return currentValue.Protocol < value[0]
			}
			return nil
		case ">":
			condition.compareValue = func(currentValue *ExtraKey) bool {
				return currentValue.Protocol > value[0]
			}
			return nil
		case "<=":
			condition.compareValue = func(currentValue *ExtraKey) bool {
				return currentValue.Protocol <= value[0]
			}
			return nil
		case ">=":
			condition.compareValue = func(currentValue *ExtraKey) bool {
				return currentValue.Protocol >= value[0]
			}
			return nil
		default:
			return errors.New("Comparator \"" + condition.comparator + "\" not allowed for attribute \"" + condition.attribute + "\"")
		}
	default:
		return errors.New("Unknown attribute \"" + condition.attribute + "\"")
	}
}

// conditionBytesAndNetmask returns the database's binary representation of the
// value of the given condition. It also validates the condition using attribute specific
// validation logic  (e.g. no IPv4 address with digits greater than 255).
//
// Input:
//   condition:    a conditionNode
//
// Output:
//   byte[]:       the value of the condition node serialized into a byte slice of the same
//                 format stored in the database.
//   int:          (Optionally) if the attribute of condition is a CIDR (e.g. 192.168.0.0/18)
//                 the length of the netmask (18 in this case)
//   error:        message whenever a condition was incorrectly specified
func conditionBytesAndNetmask(condition conditionNode) ([]byte, int, error) {

	// translate the indicated value into bytes
	var (
		err       error
		num       uint64
		isIn      bool
		netmask   int64 = 0
		condBytes []byte
	)

	attribute, comparator, value := condition.attribute, condition.comparator, condition.value

	switch comparator {
	case "=", "!=", "<", ">", "<=", ">=":
		switch attribute {
		case "dip", "sip":
			if condBytes, err = IPStringToBytes(value); err != nil {
				return nil, 0, errors.New("Could not parse IP address: " + value)
			}
		case "dnet", "snet":
			cidr := strings.Split(value, "/")
			if len(cidr) < 2 {
				return nil, 0, errors.New("Could not get netmask. Use CIDR notation. Example: 192.168.1.17/25")
			}

			// parse netmask and run sanity checks
			if netmask, err = strconv.ParseInt(cidr[1], 10, 32); err != nil {
				return nil, 0, errors.New("Failed to parse netmask " + cidr[1] + ". Use CIDR notation. Example: 192.168.1.17/25 ")
			}

			// check if the netmask is within allowed bounds
			isIPv6Address := strings.Contains(cidr[0], ":")

			if isIPv6Address {
				if netmask > 128 {
					return nil, 0, errors.New("Incorrect netmask. Maximum possible value is 128 for IPv6 networks.")
				}
			} else {
				if netmask > 32 {
					return nil, 0, errors.New("Incorrect netmask. Maximum possible value is 32 for IPv4 networks.")
				}
			}

			// get ip bytes and apply netmask
			if condBytes, err = IPStringToBytes(cidr[0]); err != nil {
				return nil, 0, errors.New("Could not parse ip address: " + value)
			}

			// zero out unused bytes of IP
			for i := (netmask + 7) / 8; i < 16; i++ {
				condBytes[i] = 0
			}
			// apply masking
			if netmask/8 < 16 {
				condBytes[netmask/8] &= uint8(0xFF) << uint8(8-(netmask%8))
			}
		case "proto":
			if num, err = strconv.ParseUint(value, 10, 8); err != nil {
				if num, isIn = GetIPProtoID(value); !isIn {
					return nil, 0, errors.New("Could not parse protocol value: " + err.Error())
				}
			}

			condBytes = []byte{uint8(num & 0xff)}
		case "dport":
			if num, err = strconv.ParseUint(value, 10, 16); err != nil {
				return nil, 0, errors.New("Could not parse dport value: " + err.Error())
			}

			condBytes = []byte{uint8(num >> 8), uint8(num & 0xff)}
		default:
			return nil, 0, errors.New("Unknown attribute: " + attribute)
		}
	default:
		return nil, 0, errors.New("Unknown comparator: " + comparator)
	}

	return condBytes, int(netmask), nil
}

// Condition conversion utility functions ------------------------------------------------
func IPStringToBytes(ip string) ([]byte, error) {
	var is_ipv4 bool = strings.Contains(ip, ".")

	ipaddr := net.ParseIP(ip)
	if len(ipaddr) == 0 {
		return nil, errors.New("IP parse: incorrect format")
	}

	if is_ipv4 {
		ipaddr[0], ipaddr[1], ipaddr[2], ipaddr[3] = ipaddr[12], ipaddr[13], ipaddr[14], ipaddr[15]
		ipaddr[12], ipaddr[13], ipaddr[14], ipaddr[15] = 0, 0, 0, 0
		ipaddr[10], ipaddr[11] = 0, 0 // Zero out v4InV6Prefix set by net.ParseIP
	}

	return ipaddr, nil
}
