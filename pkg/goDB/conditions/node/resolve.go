/////////////////////////////////////////////////////////////////////////////////
//
// resolve.go
//
// Written by Lorenz Breidenbach lob@open.ch, February 2016
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package node

import (
	"fmt"
	"net"
	"regexp"
	"sort"
	"time"

	"github.com/els0r/goProbe/pkg/types"
)

var hostnameRegexp = regexp.MustCompile(`[a-zA-Z0-9\-]+(?:\.[a-zA-Z0-9\-]+)*\.?`)

type lookupHostResult struct {
	hostname string
	addrs    []string
	err      error
}

// Returns a resolved version of node.
func resolve(node Node, timeout time.Duration) (Node, error) {
	// Find all hostnames
	hostnames := make(map[string]struct{})
	_, err := node.transform(func(node conditionNode) (Node, error) {
		// We only expect a hostname in sip or dip attributes
		if node.attribute != types.SIPName && node.attribute != types.DIPName {
			return node, nil
		}

		// For IPs we are already done.
		if net.ParseIP(node.value) != nil {
			return node, nil
		}

		// Does the value vaguely look like a hostname?
		if !hostnameRegexp.MatchString(node.value) {
			return nil, fmt.Errorf("invalid value in condition: '%s' is neither an ip nor a hostname", node.value)
		}

		hostnames[node.value] = struct{}{}

		return node, nil
	})
	if err != nil {
		return nil, err
	}

	// Resolve them asynchronously with a timeout
	timer := time.NewTimer(timeout)
	resultChan := make(chan lookupHostResult, 10)

	for hostname := range hostnames {
		hostname := hostname
		go func() {
			addrs, err := net.LookupHost(hostname)

			// always arrange the addresses such that IPv4 comes before IPv6
			sort.SliceStable(addrs, func(i, j int) bool {
				addrA := net.ParseIP(addrs[i])
				addrB := net.ParseIP(addrs[j])
				return addrA.To4() != nil && addrB.To4() == nil
			})

			resultChan <- lookupHostResult{hostname, addrs, err}
		}()
	}

	lookups := make(map[string][]string)
	for count := 0; count < len(hostnames); count++ {
		select {
		case <-timer.C:
			return nil, fmt.Errorf("timeout while resolving hostnames in conditional")
		case result := <-resultChan:
			if result.err != nil {
				return nil, result.err
			}
			lookups[result.hostname] = result.addrs
		}
	}

	// Rewrite all conditions involving hostnames to use IPs
	return node.transform(func(node conditionNode) (Node, error) {
		// We only expect a domain in sip or dip attributes
		if node.attribute != types.SIPName && node.attribute != types.DIPName {
			return node, nil
		}

		addrs, exists := lookups[node.value]
		if !exists {
			return node, nil
		}

		var conditions []Node
		for _, addr := range addrs {
			condition := newConditionNode(node.attribute, node.comparator, addr)
			conditions = append(conditions, condition)
		}

		return listToTree(false, conditions), nil
	})
}
