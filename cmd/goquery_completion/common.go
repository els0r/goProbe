/////////////////////////////////////////////////////////////////////////////////
//
// common.go
//
// Written by Lorenz Breidenbach lob@open.ch, February 2016
// Copyright (c) 2016 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package main

import "fmt"

func last(ss []string) string {
	if len(ss) > 0 {
		return ss[len(ss)-1]
	}
	return ""
}

func penultimate(ss []string) string {
	if len(ss) > 1 {
		return ss[len(ss)-2]
	}
	return ""
}

func antepenultimate(ss []string) string {
	if len(ss) > 2 {
		return ss[len(ss)-3]
	}
	return ""
}

type suggestions interface {
	suggestionsMarker()
}

type unknownSuggestions struct{}

func (unknownSuggestions) suggestionsMarker() {}

type suggestion struct {
	token         string
	tokenPlusMeta string
	accept        bool
}

type knownSuggestions struct {
	suggestions []suggestion
}

func (knownSuggestions) suggestionsMarker() {}

func complete(
	tokenize func(string) []string,
	join func([]string) string,
	next func([]string) suggestions,
	unknown func(string) []string,
	s string,
) []string {
	var completions []string

	tokens := tokenize(s)
	suggs := next(tokens)

	switch suggs := suggs.(type) {
	case unknownSuggestions:
		completions = unknown(s)
	case knownSuggestions:
		switch len(suggs.suggestions) {
		case 0:
			// do nothing
		case 1:
			sugg := suggs.suggestions[0]
			tokens[len(tokens)-1] = sugg.token
			if sugg.accept {
				completions = append(completions, join(tokens))
			}
			tokens = append(tokens, "")
			completions = append(completions, complete(tokenize, join, next, unknown, join(tokens))...)
		default:
			for _, sugg := range suggs.suggestions {
				tokens[len(tokens)-1] = sugg.tokenPlusMeta
				if sugg.accept {
					completions = append(completions, join(tokens))
				} else {
					completions = append(completions, join(append(tokens, "")))
				}
			}
		}
	default:
		panic(fmt.Sprintf("Unexpected type %T", suggs))
	}

	return completions
}
