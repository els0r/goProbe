/////////////////////////////////////////////////////////////////////////////////
//
// conditional.go
//
// Written by Lorenz Breidenbach lob@open.ch, February 2016
// Copyright (c) 2016 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package main

import (
	"strings"

	"OSAG/goDB"
)

func openParens(tokens []string) int {
	open := 0
	for _, token := range tokens {
		switch token {
		case "(":
			open++
		case ")":
			open--
		}
	}
	return open
}

func nextAll(prevprev, prev string, openParens int) []suggestion {
	s := func(sugg string, accept bool) suggestion {
		if accept {
			return suggestion{sugg, sugg, accept}
		} else {
			return suggestion{sugg, sugg + " ...  ", accept}
		}
	}

	switch prev {
	case "", "(", "&", "|":
		return []suggestion{
			s("!", false),
			s("(", false),
			s("dip", false),
			s("sip", false),
			s("dnet", false),
			s("snet", false),
			s("dst", false),
			s("src", false),
			s("host", false),
			s("net", false),
			s("dport", false),
			s("proto", false),
		}
	case "!":
		return []suggestion{
			s("(", false),
			s("dip", false),
			s("sip", false),
			s("dnet", false),
			s("snet", false),
			s("dst", false),
			s("src", false),
			s("host", false),
			s("net", false),
			s("dport", false),
			s("proto", false),
		}
	case "dip", "sip", "dnet", "snet", "dst", "src", "host", "net":
		return []suggestion{
			s("=", false),
			s("!=", false),
		}
	case "dport", "proto":
		return []suggestion{
			s("=", false),
			s("!=", false),
			s("<", false),
			s(">", false),
			s("<=", false),
			s(">=", false),
		}
	case "=", "!=", "<", ">", "<=", ">=":
		switch prevprev {
		case "proto":
			var result []suggestion
			for name := range goDB.IPProtocolIDs {
				result = append(result, suggestion{name, name + " ...", openParens == 0})
			}
			return result
		default:
			return nil
		}
	case ")":
		if openParens > 0 {
			return []suggestion{
				s(")", openParens == 1),
				s("&", false),
				s("|", false),
			}
		} else {
			return []suggestion{
				s("&", false),
				s("|", false),
			}
		}
	default:
		switch prevprev {
		case "=", "!=", "<", ">", "<=", ">=":
			if openParens > 0 {
				return []suggestion{
					s(")", openParens == 1),
					s("&", false),
					s("|", false),
				}
			} else {
				return []suggestion{
					s("&", false),
					s("|", false),
				}
			}
		default:
			return nil
		}
	}
}

func conditional(args []string) []string {
	tokenize := func(conditional string) []string {
		san, err := goDB.SanitizeUserInput(conditional)
		if err != nil {
			return nil
		}
		//fmt.Fprintf(os.Stderr, "%#v\n", san)
		tokens, err := goDB.TokenizeConditional(san)
		if err != nil {
			return nil
		}
		//fmt.Fprintf(os.Stderr, "%#v\n", tokens)

		var startedNewToken bool
		startedNewToken = len(tokens) == 0 || strings.LastIndex(conditional, tokens[len(tokens)-1])+len(tokens[len(tokens)-1]) < len(conditional)

		if startedNewToken {
			tokens = append(tokens, "")
		}

		return tokens
	}

	join := func(tokens []string) string {
		return strings.Join(tokens, " ")
	}

	next := func(tokens []string) suggestions {
		var suggs []suggestion
		for _, sugg := range nextAll(antepenultimate(tokens), penultimate(tokens), openParens(tokens)) {
			if strings.HasPrefix(sugg.token, last(tokens)) {
				suggs = append(suggs, sugg)
			}
		}
		if len(suggs) == 0 {
			return unknownSuggestions{}
		} else {
			return knownSuggestions{suggs}
		}
	}

	unknown := func(s string) []string {
		return []string{s, " (I can't help you)"}
	}

	return complete(tokenize, join, next, unknown, last(args))
}
