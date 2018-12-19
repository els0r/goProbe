/////////////////////////////////////////////////////////////////////////////////
//
// ParseConditional.go
//
// This file contains code for parsing tokenized conditionals into conditional ASTs
// For example, the conditional expression
//   {"sip", "=", "127.0.0.1", "|", "!", "(", "dport", "<", "80", ")"}
// would be parsed into an ast like this:
//                  ┌────────┐
//                  │   OR   │
//                  └────────┘
//                   ╱      ╲
//                  ╱        ╲
//                 ╱          ╲
//                ╱            ╲
//               ╱              ╲
// ┌──────────────────┐        ┌──────────────────┐
// │    CONDITION     │        │    CONDITION     │
// │ attribute: sip   │        │ attribute: dport │
// │ comparator: =    │        │ comparator: >=   │
// │ value: 127.0.0.1 │        │ value: 80        │
// └──────────────────┘        └──────────────────┘
//
// Written by Lennart Elsen      lel@open.ch and
//            Lorenz Breidenbach lob@open.ch, October 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package goDB

import "fmt"

// Parses the given conditional into an AST.
//
// Note that certain validity checks only take time during instrumentation.
// For example, parseConditional("sip = 300.300.300.300.300") won't throw an
// error because it treats the ip as a string. Only during the Node.instrument()
// an error will be thrown.
func parseConditional(tokens []string) (conditionalNode Node, err error) {
	if len(tokens) == 0 {
		return nil, nil
	}

	p := newParser(tokens)
	conditionalNode = p.conditional()
	if !p.success() {
		return nil, p.err
	} else if !p.eof() {
		p.die("Input unexpectyly continues")
		return nil, p.err
	}

	return conditionalNode, nil
}

// A parser for conditionals such as "!(sip == 127.0.01 | dip != 10.0.0.1)"
// Call the parser.Parse method to parse a token stream.
//
// The parser is implemented using a recursive descent technique.
// We first left-factor the condition grammar so we get an equivalent grammar
// that contains no left recursion.
//     conditional -> disjunction
//     disjunction -> conjunction ('|' conjunction)*
//     conjunction -> negation ('&' negation)*
//     negation -> '!' primitive | primitive
//     primitive -> '(' disjunction ')' | condition
//     condition -> attribute comparator value
//     comparator -> '=' | '!=' | '<' | '>' | '<=' | '>='
// (Terminal symbols are written in single quotes)
// (A rule part written with a star is meant to be repeated zero or more times)
// We observe that this grammar is in LL(1), i.e. the parser can always decide which
// production it should use by looking ahead a single token.
// As a result we can translate the grammar into code almost one-to-one; furthermore,
// the resulting parser runs in O(n).
type parser struct {
	// The token stream on which the parser operates
	tokens []string
	// The parser's current position in the token stream.
	// (Represented by an index for the tokens slice)
	pos int
	// If a parsing error occurred, contains the error message.
	err error
}

// Creates a new parser for the given token stream.
func newParser(tokens []string) parser {
	return parser{
		tokens: tokens,
		pos:    0,
		err:    nil,
	}
}

// Indicates whether the parser succeeded so far
func (p parser) success() bool {
	return p.err == nil
}

// Creates a parser error with the given description and a nice error
// message pointing to current token in the token stream.
// Example of an error message created by this method:
//        ( sip = 192.168.1.1
//                            ^
//        Expected ), but didn't get it.
//
func (p *parser) die(description string, args ...interface{}) {
	// Reassemble the tokens.
	final := ""
	for i := 0; i < p.pos; i++ {
		final += p.tokens[i] + " "
	}
	// Remember position of current token in reassembled string
	offset := len(final)
	// Add remaining tokens
	for i := p.pos; i < len(p.tokens); i++ {
		final += p.tokens[i] + " "
	}
	final += "\n"
	// Draw arrow
	for i := 0; i < offset; i++ {
		final += " "
	}
	final += "^\n"
	// Add error description
	final += description
	p.err = fmt.Errorf(final, args...)
}

// Returns the token at the current position in the token stream
// and advances the parser's position in the token stream by one.
func (p *parser) advance() (result string) {
	if p.eof() {
		p.die("Unexpected end of input")
		return ""
	}
	result = p.tokens[p.pos]
	p.pos += 1
	return
}

// Indicates whether the end of the token stream has been reached.
func (p *parser) eof() bool {
	return p.pos >= len(p.tokens)
}

// Advances the parser's position in the token stream and returns true,
// if the current token in the token stream equals the token argument.
// Otherwise, the parser stays at its current position and returns false.
func (p *parser) accept(token string) bool {
	if !p.eof() && p.tokens[p.pos] == token {
		p.advance()
		return true
	} else {
		return false
	}
}

// Like accept, but the parse fails if the argument token doesn't equal the current token.
func (p *parser) expect(token string) {
	if p.accept(token) {
	} else {
		p.die("Expected %v, but didn't get it.\n", token)
		return
	}
}

// Corresponds to grammar rule "conditional"
func (p *parser) conditional() Node {
	return p.disjunction()
}

// Converts a list of nodes into a right-hanging tree of andNodes (if the and
// argument is true) or a right-hanging tree of orNodes (if the and argument is false).
// Example:
// listToTree(true, []Node{A,B,C}) produces
//      [&]
//       /\
//      /  \
//     A   [&]
//          /\
//         /  \
//        B    C
//
// listToTree(false, []Node{A,B,C}) produces
//      [|]
//       /\
//      /  \
//     A   [|]
//          /\
//         /  \
//        B    C
//
// If nodes[] doesn't contain any element of type *Node, the resulting tree
// also won't contain any elements of type *Node.
func listToTree(and bool, nodes []Node) (result Node) {
	if len(nodes) == 0 {
		panic("nodes must not be empty")
	}

	if len(nodes) == 1 {
		return nodes[0]
	}

	if and {
		return andNode{
			left:  nodes[0],
			right: listToTree(and, nodes[1:]),
		}
	} else {
		return orNode{
			left:  nodes[0],
			right: listToTree(and, nodes[1:]),
		}
	}
}

// Corresponds to grammar rule "disjunction"
func (p *parser) disjunction() (result Node) {
	nodes := []Node{p.conjunction()}
	if !p.success() {
		return
	}
	for p.accept("|") {
		if !p.success() {
			return
		}
		nodes = append(nodes, p.conjunction())
		if !p.success() {
			return
		}
	}
	result = listToTree(false, nodes)
	return
}

// Corresponds to grammar rule "conjunction"
func (p *parser) conjunction() (result Node) {
	nodes := []Node{p.negation()}
	if !p.success() {
		return
	}
	for p.accept("&") {
		if !p.success() {
			return
		}
		nodes = append(nodes, p.negation())
		if !p.success() {
			return
		}
	}
	result = listToTree(true, nodes)
	return
}

// Corresponds to grammar rule "negation"
func (p *parser) negation() (result Node) {
	if p.accept("!") {
		if !p.success() {
			return
		}
		result = notNode{node: p.primitive()}
	} else {
		if !p.success() {
			return
		}
		result = p.primitive()
	}
	return
}

// Corresponds to grammar rule "primitive"
func (p *parser) primitive() (result Node) {
	if p.accept("(") {
		if !p.success() {
			return
		}
		result = p.disjunction()
		if !p.success() {
			return
		}
		p.expect(")")
	} else {
		if !p.success() {
			return
		}
		result = p.condition()
	}
	return
}

// Corresponds to grammar rule "condition"
func (p *parser) condition() (result Node) {
	var condition conditionNode
	condition.attribute = p.attribute()
	if !p.success() {
		return
	}
	condition.comparator = p.comparator()
	if !p.success() {
		return
	}
	condition.value = p.value()
	result = condition
	return
}

// Corresponds to grammar rule "attribute"
func (p *parser) attribute() (result string) {
	attributes := []string{
		"dip", "sip", "dnet", "snet", "dport", "proto", // non-sugar
		"dst", "src", "host", "net", // sugar
	}
	for _, attrib := range attributes {
		if p.accept(attrib) {
			if !p.success() {
				return
			}
			return attrib
		}
	}

	p.die("Expected attribute")
	return
}

// Corresponds to grammar rule "comparator"
func (p *parser) comparator() (result string) {
	if p.accept("=") {
		if !p.success() {
			return
		}
		result = "="
	} else if p.accept("!=") {
		if !p.success() {
			return
		}
		result = "!="
	} else if p.accept("<=") {
		if !p.success() {
			return
		}
		result = "<="
	} else if p.accept(">=") {
		if !p.success() {
			return
		}
		result = ">="
	} else if p.accept("<") {
		if !p.success() {
			return
		}
		result = "<"
	} else if p.accept(">") {
		if !p.success() {
			return
		}
		result = ">"
	} else {
		p.die("Expected comparison operator")
	}
	return
}

// Corresponds to grammar rule "value"
func (p *parser) value() (result string) {
	result = p.advance()
	return
}
