/////////////////////////////////////////////////////////////////////////////////
//
// ifaces.go
//
// Written by Lorenz Breidenbach lob@open.ch, February 2016
// Copyright (c) 2016 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package main

import (
	"fmt"
	"strings"

	"github.com/els0r/goProbe/pkg/goDB/info"
	"github.com/els0r/goProbe/pkg/util"
)

// tries to find the db path based on args
// If no db path has been specified, returns the default DB path.
func dbPath(args []string) string {
	result := defaultDBPath
	minusd := false
	for _, arg := range args {
		switch {
		case arg == "-d":
			minusd = true
		case minusd:
			minusd = false
			result = arg
		}
	}
	return result
}

func ifaces(args []string) []string {
	tokenize := func(qt string) []string {
		return strings.Split(qt, ",")
	}

	join := func(attribs []string) string {
		return strings.Join(attribs, ",")
	}

	dbpath := dbPath(args)

	dbIfaces, err := info.GetInterfaces(dbpath)
	if err != nil {
		return nil
	}

	tunnels := util.TunnelInfos()

	next := func(ifaces []string) suggestions {
		used := map[string]struct{}{}
		for _, iface := range ifaces[:len(ifaces)-1] {
			used[iface] = struct{}{}
		}

		var suggs []suggestion

		if len(ifaces) == 1 && strings.HasPrefix("any", strings.ToLower(last(ifaces))) {
			suggs = append(suggs, suggestion{"ANY", "ANY (query all interfaces)", true})
		} else {
			for _, iface := range ifaces {
				if strings.ToLower(iface) == "any" {
					return knownSuggestions{[]suggestion{}}
				}
			}
		}

		for _, iface := range dbIfaces {
			if _, used := used[iface]; !used && strings.HasPrefix(iface, last(ifaces)) {
				if info, isTunnel := tunnels[iface]; isTunnel {
					suggs = append(suggs, suggestion{iface, fmt.Sprintf("%s (%s: %s)   ", iface, info.PhysicalIface, info.Peer), true})
				} else {
					suggs = append(suggs, suggestion{iface, iface, true})
				}
			}
		}

		return knownSuggestions{suggs}
	}

	unknown := func(_ string) []string {
		panic("There are no unknown suggestions for interfaces.")
	}

	return complete(tokenize, join, next, unknown, last(args))
}
