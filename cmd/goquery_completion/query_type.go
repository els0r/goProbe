/////////////////////////////////////////////////////////////////////////////////
//
// query_type.go
//
// Written by Lorenz Breidenbach lob@open.ch, February 2016
// Copyright (c) 2016 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package main

import "strings"

func queryType(args []string) []string {
	tokenize := func(qt string) []string {
		return strings.Split(qt, ",")
	}

	join := func(attribs []string) string {
		return strings.Join(attribs, ",")
	}

	unusedAttribs := func(attribs []string) []string {
		attribUnused := map[string]bool{
			"time":  true,
			"iface": true,
			"sip":   true,
			"dip":   true,
			"dport": true,
			"proto": true,
		}

		for _, attrib := range attribs {
			switch attrib {
			case "talk_conv", "talk_src", "talk_dst", "apps_port", "agg_talk_port", "raw":
				return nil
			case "src":
				attrib = "sip"
			case "dst":
				attrib = "dip"
			}
			attribUnused[attrib] = false
		}

		var result []string
		for attrib, unused := range attribUnused {
			if unused {
				result = append(result, attrib)
			}
		}
		return result
	}

	next := func(attribs []string) suggestions {
		var suggs []suggestion
		if len(attribs) == 1 {
			for _, qt := range []string{"talk_conv", "talk_src", "talk_dst", "apps_port", "agg_talk_port", "raw"} {
				if strings.HasPrefix(qt, attribs[0]) {
					suggs = append(suggs, suggestion{qt, qt, true})
				}
			}
		}
		for _, ua := range unusedAttribs(attribs[:len(attribs)-1]) {
			if strings.HasPrefix(ua, last(attribs)) {
				suggs = append(suggs, suggestion{ua, ua, true})
			}
		}
		return knownSuggestions{suggs}
	}

	unknown := func(_ string) []string {
		panic("There are no unknown suggestions for the query type.")
	}

	return complete(tokenize, join, next, unknown, last(args))
}
