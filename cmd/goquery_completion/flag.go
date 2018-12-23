/////////////////////////////////////////////////////////////////////////////////
//
// flag.go
//
// Written by Lorenz Breidenbach lob@open.ch, February 2016
// Copyright (c) 2016 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package main

import "strings"

var flags = map[string]suggestion{
	"-a":               {"-a", "-a (sort ascending)", true},
	"-c":               {"-c", "-c <condition>", true},
	"-d":               {"-d", "-d <db path>", true},
	"-e":               {"-e", "-e <output format>", true},
	"-f":               {"-f", "-f <start time>", true},
	"-l":               {"-l", "-l <end time>", true},
	"-h":               {"-h", "-h (show help)", true},
	"-help":            {"-help", "-help (show help)", true},
	"-i":               {"-i", "-i <interface(s)>", true},
	"-in":              {"-in", "-in (only incoming)", true},
	"-list":            {"-list", "-list (list interfaces)", true},
	"-n":               {"-n", "-n <# of results to print>", true},
	"-out":             {"-out", "-out (only outgoing)", true},
	"-resolve":         {"-resolve", "-resolve (run RDNS)", true},
	"-resolve-rows":    {"-resolve-rows", "-resolve-rows", true},
	"-resolve-timeout": {"-resolve-timeout", "-resolve-timeout", true},
	"-s":               {"-s", "-s <sort by>", true},
	"-sum":             {"-sum", "-sum (sum incoming & outgoing)", true},
}

func flag(args []string) []string {
	tokenize := func(flag string) []string {
		return []string{flag}
	}

	join := func(tokens []string) string {
		return strings.Join(tokens, " ")
	}

	next := func(tokens []string) suggestions {
		unusedFlags := map[suggestion]struct{}{}
		for _, flag := range flags {
			unusedFlags[flag] = struct{}{}
		}

		for _, arg := range args[:len(args)-1] {
			if strings.HasPrefix(arg, "-") {
				delete(unusedFlags, flags[arg])
				// {-in, -out} and -sum are mutually exclusive
				switch arg {
				case "-in", "-out":
					delete(unusedFlags, flags["-sum"])
				case "-sum":
					delete(unusedFlags, flags["-in"])
					delete(unusedFlags, flags["-out"])
				}
			}
		}

		var suggs []suggestion
		for sugg := range unusedFlags {
			if strings.HasPrefix(sugg.token, last(tokens)) {
				suggs = append(suggs, sugg)
			}
		}
		return knownSuggestions{suggs}
	}

	unknown := func(_ string) []string {
		panic("There are no unknown suggestions for flag completion.")
	}

	return complete(tokenize, join, next, unknown, last(args))
}
