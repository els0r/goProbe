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

	"github.com/els0r/goProbe/v4/pkg/goDB/conditions"
	"github.com/els0r/goProbe/v4/pkg/goDB/protocols"
	"github.com/els0r/goProbe/v4/pkg/types"
)

func openParens(tokens []string) (open int) {
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

// dirKeywordCount returns the number of dir keyword occurrences in tokens.
func dirKeywordCount(tokens []string) (count int) {
	for _, token := range tokens {
		if token == types.FilterKeywordDirection || token == types.FilterKeywordDirectionSugared {
			count++
		}
	}
	return count
}

// firstUnnestedBinaryOp returns the position of the first 'unnested' binary logical
// operator inside tokens and the respective token.
// It returns -1 and an empty string if tokens do not contain any binary logical operator.
func firstUnnestedBinaryOp(tokens []string) (int, string) {
	for i, token := range tokens {
		if (token == "&" || token == "|") && openParens(tokens[:i]) == 0 {
			return i, token
		}
	}
	return -1, ""
}

// unnestedBinaryOpCount returns the number of 'unnested' binary operators within tokens.
func unnestedBinaryOpCount(tokens []string) (count int) {
	for i, token := range tokens {
		if (token == "&" || token == "|") && openParens(tokens[:i]) == 0 {
			count++
		}
	}
	return count
}

// conditionStringValid checks if the condition string is valid,
// in which case suggestions have to be provided.
// Returns true if the condition string is valid, and false otherwise.
func conditionStringValid(tokens []string) bool {
	dirKeywordCount := dirKeywordCount(tokens)
	firstUnnestedBinaryOpPos, firstUnnestedBinaryOp := firstUnnestedBinaryOp(tokens)
	unnestedBinaryOpCount := unnestedBinaryOpCount(tokens)

	// If the dir keyword occurs more than once, the condition is invalid.
	if dirKeywordCount > 1 {
		return false
	}

	// If the dir keyword occurs exactly once but there's more than
	// one 'unnested' binary operator, the condition is invalid.
	if dirKeywordCount == 1 && unnestedBinaryOpCount > 1 {
		return false
	}

	// Get position of dir keyword within the condition string.
	var dirKeywordPos = -1
	for i, token := range tokens {
		if token == types.FilterKeywordDirection || token == types.FilterKeywordDirectionSugared {
			dirKeywordPos = i
			break
		}
	}

	// If the dir keyword already occurs, and the first 'unnested' binary operator is
	// not an &, the condition string is invalid.
	if dirKeywordCount > 0 && firstUnnestedBinaryOpPos >= 0 && firstUnnestedBinaryOp != "&" {
		return false
	}

	// If the dir keyword already occurs (but not directly in the beginning of the
	// condition string) and doesn't directly follow the top-level &,
	// the condition string is invalid.
	if dirKeywordPos > 0 && dirKeywordPos != firstUnnestedBinaryOpPos+1 {
		return false
	}

	return true
}

func nextAll(prevprev, prev string, openParens int) []suggestion {
	s := func(sugg string, accept bool) suggestion {
		if accept {
			return suggestion{sugg, sugg, accept}
		}
		return suggestion{sugg, sugg + " ...  ", accept}
	}

	switch prev {
	case "", "(", "&", "|":
		return []suggestion{
			s("!", false),
			s("(", false),
			s(types.DIPName, false),
			s(types.SIPName, false),
			s("dnet", false),
			s("snet", false),
			s("dst", false),
			s("src", false),
			s("host", false),
			s("net", false),
			s(types.DportName, false),
			s("port", false),
			s(types.ProtoName, false),
			s(types.FilterKeywordDirection, false),
			s(types.FilterKeywordDirectionSugared, false),
		}
	case "!":
		return []suggestion{
			s("(", false),
			s(types.DIPName, false),
			s(types.SIPName, false),
			s("dnet", false),
			s("snet", false),
			s("dst", false),
			s("src", false),
			s("host", false),
			s("net", false),
			s(types.DportName, false),
			s("port", false),
			s(types.ProtoName, false),
		}
	case types.DIPName, types.SIPName, "dnet", "snet", "dst", "src", "host", "net":
		return []suggestion{
			s("=", false),
			s("!=", false),
		}
	case types.DportName, "port", types.ProtoName:
		return []suggestion{
			s("=", false),
			s("!=", false),
			s("<", false),
			s(">", false),
			s("<=", false),
			s(">=", false),
		}
	case types.FilterKeywordDirection, types.FilterKeywordDirectionSugared:
		return []suggestion{
			s("=", false),
		}
	case "=", "!=", "<", ">", "<=", ">=":
		switch prevprev {
		case types.ProtoName:
			var result []suggestion
			for name := range protocols.IPProtocolIDs {
				result = append(result, suggestion{name, name + " ...", openParens == 0})
			}
			return result
		case types.FilterKeywordDirection, types.FilterKeywordDirectionSugared:
			var result []suggestion
			for _, direction := range types.DirectionFilters {
				result = append(result, s(direction, true))
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
		}
		return []suggestion{
			s("&", false),
			s("|", false),
		}
	case types.FilterTypeDirectionIn, types.FilterTypeDirectionOut,
		types.FilterTypeDirectionUni, types.FilterTypeDirectionBi,
		types.FilterTypeDirectionInSugared, types.FilterTypeDirectionOutSugared,
		types.FilterTypeDirectionUniSugared, types.FilterTypeDirectionBiSugared:
		return []suggestion{
			s("&", false),
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
			}
			return []suggestion{
				s("&", false),
				s("|", false),
			}
		default:
			return nil
		}
	}
}

func conditional(args []string) []string {
	tokenize := func(conditional string) []string {
		tokens, err := conditions.Tokenize(conditions.SanitizeUserInput(conditional))
		if err != nil {
			return nil
		}

		if startedNewToken := len(tokens) == 0 || strings.LastIndex(conditional, tokens[len(tokens)-1])+len(tokens[len(tokens)-1]) < len(conditional); startedNewToken {
			tokens = append(tokens, "")
		}

		return tokens
	}

	join := func(tokens []string) string {
		return strings.Join(tokens, " ")
	}

	next := func(tokens []string) suggestions {
		var suggs []suggestion

		// Only provide suggestions for currently valid condition strings.
		if conditionStringValid(tokens) {
			prevprev := antepenultimate(tokens)
			prev := penultimate(tokens)
			openParens := openParens(tokens)
			last := last(tokens)
			dirKeywordCount := dirKeywordCount(tokens[:len(tokens)-1])
			for _, sugg := range nextAll(prevprev, prev, openParens) {
				if strings.HasPrefix(sugg.token, last) {
					// Check if suggestion is valid based on the current condition string.
					if verifySuggestion(sugg, tokens, prev, openParens, dirKeywordCount) {
						suggs = append(suggs, sugg)
					}
				}
			}
			// Check if termination of condition string must be enforced in order to be valid.
			if conditionMustTerminate(suggs, prevprev, prev, last, openParens, dirKeywordCount, len(tokens)) {
				return knownSuggestions{[]suggestion{{token: quit, accept: false}}}
			}
		}

		if len(suggs) == 0 {
			return unknownSuggestions{}
		}
		return knownSuggestions{suggs}
	}

	unknown := func(s string) []string {
		return []string{s, " (I can't help you)"}
	}

	return complete(tokenize, join, next, unknown, last(args))
}

// verifySuggestion enforces some structural constraints on the condition,
// only if the current suggestion is the dir keyword, or the dir keyword
// has already appeared, ensuring that the dir keyword only occurs at valid places.
func verifySuggestion(sugg suggestion, tokens []string, prev string, openParens, dirKeywordCount int) bool {
	if strings.Contains(sugg.token, types.FilterKeywordDirection) ||
		strings.Contains(sugg.token, types.FilterKeywordDirectionSugared) ||
		dirKeywordCount > 0 {
		// Filter out invalid suggestions.
		if !checkDirKeywordConstraints(sugg, tokens, prev, openParens, dirKeywordCount) {
			return false
		}
	}
	return true
}

// checkDirKeywordConstraints checks whether sugg is a valid suggestion with respect to the
// constraints on the structure of the condition string introduced by the dir keyword.
// Returns true if sugg is a valid suggestion and false otherwise.
func checkDirKeywordConstraints(sugg suggestion, tokens []string, prev string, openParens, dirKeywordCount int) bool {
	firstUnnestedBinaryOpPos, firstUnnestedBinaryOp := firstUnnestedBinaryOp(tokens)
	nTokens := len(tokens)
	// Determine whether prev is a top-level conjunction.
	topLevelAnd := firstUnnestedBinaryOp == "&" && firstUnnestedBinaryOpPos == nTokens-2
	// Determine whether a top-level conjunction has previously occurred.
	topLevelAndOccurred := firstUnnestedBinaryOp == "&" && firstUnnestedBinaryOpPos <= nTokens-2

	// The dir keyword must only be suggested at the start of the condition (condition string currently empty)
	// or directly after a top-level conjunction.
	if !strings.Contains(sugg.token, "directional") &&
		(strings.Contains(sugg.token, types.FilterKeywordDirection) || strings.Contains(sugg.token, types.FilterKeywordDirectionSugared)) {
		if !(prev == "" || (topLevelAnd && dirKeywordCount == 0)) {
			return false
		}
	}

	// An 'unnested' disjunction is disallowed if the dir keyword is already present.
	if sugg.token == "|" && dirKeywordCount > 0 && openParens == 0 {
		return false
	}

	// If dir keyword has already occurred and there is already a top-level conjunction,
	// no other 'unnested' conjunction or disjunction is allowed
	//(otherwise dir keyword would not be part of the top-level conjunct anymore).
	if sugg.token == "&" && dirKeywordCount > 0 && topLevelAndOccurred && openParens == 0 {
		return false
	}

	return true
}

// conditionMustTerminate checks whether the condition string must be terminated now in order to be a valid condition.
// Returns true if condition string must be terminated and false otherwise.
func conditionMustTerminate(suggs []suggestion, prevprev, prev, last string, openParens, dirKeywordCount, nTokens int) bool {
	// If there are no suggestions, and there is already a previous condition,
	// and the current condition is a direction filter, the condition string
	// must end (no other conditions are allowed afterwards).
	//
	// Note: This case handles a dir keyword occurring
	// on the right of a top-level conjunction (potentially
	// with trailing whitespaces).
	// (e.g. terminates "sip = 127.0.0.1 & dir = in ",
	// "sip = 127.0.0.1 & dir = inbound ")
	if len(suggs) == 0 && nTokens > 3 {
		for _, direction := range types.DirectionFilters {
			// If dir is specified on the right side of a top-level conjunction,
			// do not allow any trailing tokens.
			if prev == direction && last == "" || last == direction {
				return true
			}
		}
	}

	// If there is exactly one suggestion, and the current
	// condition is a direction filter and this suggestion is
	// exactly what the user specified, the condition string must end.
	//
	// Note: This handles termination of condition strings ending
	// with a direction filter in case there no trailing whitespaces.
	// (e.g., terminates "sip = 127.0.0.1 & dir = inbound",
	// "sip = 127.0.0.1 & dir = unidirectional" )
	if len(suggs) == 1 && nTokens > 3 {
		for _, direction := range types.DirectionFilters {
			if suggs[0].token == direction && suggs[0].token == last {
				return true
			}
		}
	}

	// If there are no suggestions, and the dir keyword
	// has already occurred, and the current position is
	// directly after a top level condition with non-empty value,
	// the condition string must end.
	// The condition value is only validated for direction filters.
	// (e.g. terminates "dir = in & sip = 127.0.0.1", BUT also
	// "dir = in & sip = 42")
	if len(suggs) == 0 && dirKeywordCount > 0 && openParens == 0 && nTokens > 3 {
		// for direction filters, validate value
		if prevprev == types.FilterKeywordDirection || prevprev == types.FilterKeywordDirectionSugared {
			for _, direction := range types.DirectionFilters {
				if last == direction {
					return true
				}
			}
			return false
		}

		// Ensure that the condition has a non-empty value.
		switch prev {
		// Do not auto terminate on incomplete direction filters.
		case types.FilterKeywordDirection, types.FilterKeywordDirectionSugared:
			return false
		case "=", "!=", "<", ">", "<=", ">=", "&", "|":
			if last == "" {
				return false
			}
		}

		return true
	}

	// If none of the filters apply, do not auto-terminate.
	return false
}
