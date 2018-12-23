/////////////////////////////////////////////////////////////////////////////////
//
// TokenizeConditional.go
//
// This file contains code for tokenizing conditionals into string slices.
// For example, the conditional expression
//   "sip = 127.0.0.1 | !(dport < 80)"
// would be tokenized into
//   {"sip", "=", "127.0.0.1", "|", "!", "(", "dport", "<", "80", ")"}.
//
// Written by Lennart Elsen      lel@open.ch and
//            Lorenz Breidenbach lob@open.ch, October 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package goDB

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"
)

// SanitizeUserInput sanitizes a conditional string provided by the user. Its main purpose
// is to convert other forms of precedence and logical operators to the condition grammar
// used.
// For example, some people may prefer a more verbose forms such as "dport=443 or dport=8080"
// or exotic forms such as "{dport=443 || dport=8080}". These should be caught and converted
// to the grammar-conforming expression "(dport=443|dport=8080)"
//
// Input:
//  conditional: string containing the conditional specified in "user grammar"
//
// Output:
//  string:  conditional string in the condition grammar. Note that this may still
//           include syntactical errors or malspecified conditions. These will be caught
//           at a latter stage
//  error:   any error from golang's regex module
//
// NOTE:  the current implementation of GPDPIProtocols.go has to make sure that the map keys
//        of "proto" to numbers are all lower case
func SanitizeUserInput(conditional string) (string, error) {

	var (
		sanitized string
		r         *regexp.Regexp
		err       error
	)

	// expressions that count as "user grammar" for the different parts of the conditional
	var grammarConversionMap = map[string][]string{
		"!":  []string{"(^|\\s+)not\\s+"},
		"!(": []string{"(^|\\s+)not[\\(\\[\\{]"}, // Users should be able to write "not{dport = 80}"
		"&":  []string{"&&", "\\s+and\\s+", "\\*"},
		"|":  []string{"\\|\\|", "\\s+or\\s+", "\\+"},
		"(":  []string{"\\{", "\\["},
		")":  []string{"\\}", "\\]"},
		"=":  []string{"\\s+eq\\s+", "\\s+\\-eq\\s+", "\\s+equals\\s+", "===", "=="},
		"!=": []string{"\\s+neq\\s+", "\\s+-neq\\s+", "\\s+ne\\s+", "\\s+\\-ne\\s+"},
		"<=": []string{"\\s+le\\s+", "\\s+\\-le\\s+", "\\s+leq\\s+", "\\s+-leq\\s+"},
		">=": []string{"\\s+ge\\s+", "\\s+\\-ge\\s+", "\\s+geq\\s+", "\\s+-geq\\s+"},
		">":  []string{"\\s+g\\s+", "\\s+\\-g\\s+", "\\s+gt\\s+", "\\s+\\-gt\\s+", "\\s+greater\\s+"},
		"<":  []string{"\\s+l\\s+", "\\s+\\-l\\s+", "\\s+lt\\s+", "\\s+\\-lt\\s+", "\\s+less\\s+"},
	}

	// first, convert everything to lower case
	r, err = regexp.Compile(".*")
	if err != nil {
		return sanitized, err
	}

	sanitized = string(r.ReplaceAllFunc([]byte(conditional), bytes.ToLower))

	// range over map to convert the individual entries
	for condGrammarOp, userGrammarOps := range grammarConversionMap {
		for _, userOp := range userGrammarOps {
			r, err = regexp.Compile(userOp)
			if err != nil {
				return sanitized, err
			}

			sanitized = r.ReplaceAllString(sanitized, condGrammarOp)
		}
	}

	return sanitized, err
}

func startsDelimiter(char byte) bool {
	switch char {
	case '!', '=', '<', '>', '|', '&', '(', ')', ' ', '\n', '\r', '\t':
		return true
	default:
		return false
	}
}

func endsDelimiter(char byte) bool {
	return char == '='
}

// delimiterSplitFunc is the SplitFunc for delimiter tokens. Since delimiter tokens
// can never be longer than two characters it inspects at most two characters.
// All delimiter tokens apart from "!=", "<=", and ">=" are only one character long.
// For all tokens of length one that aren't prefixes of the aforementioned three
// tokens simply looking at the first byte of the token is enough to tokenize it.
// For the other six tokens ("<", ">", "!", "<=", ">=", "!=") we can simply look ahead
// one character to determine what kind of token we are dealing with.
func delimiterSplitFunc(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if len(data) == 0 {
		return
	}

	switch data[0] {
	case '=', '|', '&', '(', ')':
		advance = 1
		token = data[0:1]
		return
	case ' ', '\n', '\r', '\t':
		advance = 1
		token = []byte{' '}
		return
	default:
		if atEOF {
			advance = 1
			token = data[0:1]
			return
		}
		if len(data) < 2 {
			return 0, nil, nil
		}
		if endsDelimiter(data[1]) {
			advance = 2
			token = data[0:2]
			return
		} else {
			advance = 1
			token = data[0:1]
			return
		}
	}
}

// wordSplitFunc is the SplitFunc for word tokens. It adds characters to its output token
// until it encounters the start of a delimiter or the EOF.
func wordSplitFunc(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if len(data) == 0 {
		return
	}

	for i := range data {
		if startsDelimiter(data[i]) {
			token = data[:advance]
			return
		} else {
			advance++
		}
	}
	if atEOF {
		token = data[:advance]
		return
	}
	return 0, nil, nil
}

// Split function for tokenization of the conditionalData. (For more info, see bufio.SplitFunc)
// The conditional grammar consits of two types of tokens:
// * Word tokens are attribute names (e.g. "sip" or "dnet"), protocol names (e.g. "UDP")
//   numbers, ip addresses (e.g. "fe80::abcd:ce23"), and CIDR records (e.g. "10.0.0.0/8").
// * Delimiter tokens delimit other tokens (word tokens and delimiter tokens). Delimiter tokens
//   consist of all logical operators, comparison operators, parentheses, and white space characters.
func conditionalSplitFunc(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if len(data) == 0 {
		return
	}

	// Our grammar is simple, so we can check whether we are dealing with a delimiter
	// by looking at the first character.
	if startsDelimiter(data[0]) {
		return delimiterSplitFunc(data, atEOF)
	} else {
		return wordSplitFunc(data, atEOF)
	}
}

// TokenizeConditional tokenizes the given conditional. Note that the tokenization is "loose":
// All valid conditionals will be correctly tokenized, but there are invalid conditionals that
// will also be tokenized. Its the parser's job to catch those.
// Whitespace in conditionals is only useful for tokenization and not needed afterwards.
// TokenizeConditional doesn't emit any whitespace tokens.
//
// Limitations: Only ASCII is supported. May give strange results on fancy Unicode strings.
func TokenizeConditional(condExpression string) ([]string, error) {
	var condTokens []string

	s := bufio.NewScanner(strings.NewReader(condExpression))

	split := conditionalSplitFunc
	s.Split(split)

	for s.Scan() {
		tok := string(s.Bytes())
		if tok != " " {
			condTokens = append(condTokens, tok)
		}
	}
	if err := s.Err(); err != nil {
		return condTokens, err
	}

	return condTokens, nil
}
