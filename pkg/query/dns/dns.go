/////////////////////////////////////////////////////////////////////////////////
//
// dns.go
//
// Provides functionality for reverse DNS lookups used by goQuery.
//
// Written by Lorenz Breidenbach lob@open.ch, October 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

// Package dns provides reverse lookup functionality for goQuery's results
package dns

import (
	"net"
	"time"
)

type LookupResult struct {
	Success bool
	IP      string
	Domain  string
}

// Performs a reverse lookup on the given ips. The lookup takes at most timeout time, afterwards
// it is aborted.
// Returns a mapping IP => domain. If the lookup is aborted because of a timeout, the current mapping
// is returned with the pending lookups missing. If there is no RDNS entry for an IP, the corresponding
// key in the result will not be associated with any value (i.e. domain).
func TimedReverseLookup(ips []string, timeout time.Duration) (ipToDomain map[string]string) {
	// Compute set of ips so we look up each unique IP exactly once
	// This assumes that the ips are provided in a normalized format.
	ipToDomain = make(map[string]string)
	ipset := make(map[string]struct{})
	for _, ip := range ips {
		ipset[ip] = struct{}{}
	}

	lookupChannel := make(chan LookupResult, 1)
	var pending int
	// Perform an asynchronous lookup for every ip in the set. The results are sent
	// over the lookup channel.
	for ip, _ := range ipset {
		go func(ip string) {
			lookupR := LookupResult{}
			lookupR.IP = ip
			lookupR.Domain = ""
			domains, err := net.LookupAddr(ip)
			if err != nil {
				lookupChannel <- lookupR
			}
			if len(domains) > 0 {
				lookupR.Success = true
				lookupR.Domain = domains[0]
			}
			lookupChannel <- lookupR
		}(ip)
		pending++
	}
	for pending != 0 {
		// Aggregate results while waiting for timeout.
		select {
		case LookupResult := (<-lookupChannel):
			pending--
			if LookupResult.Success {
				ipToDomain[LookupResult.IP] = LookupResult.Domain
			}
		case <-time.After(timeout):
			pending = 0
		}
	}
	return
}
