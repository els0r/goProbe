/////////////////////////////////////////////////////////////////////////////////
//
// cmd.go
//
// Written by Lorenz Breidenbach lob@open.ch, February 2016
// Copyright (c) 2016 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

// Utility to enable bash completion for goQuery.
//
// goQuery has extensive support for bash autocompletion. To enable autocompletion,
// you need to tell bash that it should use the `goquery_completion` program for
// completing `goquery` commands.
//
// How to do this depends on your distribution.
//
// On Debian derivatives, we suggest creating a file `goquery` in `/etc/bash_completion.d` with the following contents:
//
//     _goquery() {
//         case "$3" in
//             -d) # the -d flag specifies the database directory.
//                 # we rely on bash's builtin directory completion.
//                 COMPREPLY=( $( compgen -d -- "$2" ) )
//             ;;
//
//             *)
//                 if [ -x /usr/local/share/goquery_completion ]; then
//                     mapfile -t COMPREPLY < <( /usr/local/share/goquery_completion bash "${COMP_POINT}" "${COMP_LINE}" )
//                 fi
//             ;;
//         esac
//     }
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/els0r/goProbe/pkg/version"
)

type bashMode byte

const (
	bashmodeNormal bashMode = iota
	bashmodeSingleQuote
	bashmodeDoubleQuote
)

// nobody likes you, bash. nobody! you suck!
//
// bashUnescape unescapes the given string according to bash's escaping rules
// for autocompletion. Note: the rules for escaping during completion seem
// to differ from those during 'normal' operation of the shell.
// For example, `'hello world''hello world'` is treated as ["hello world", "hello world"]
// during completion but would usually be treated as ["hello worldhello world"].
//
// weird is set to true iff we are at a weird position:
// A weird position is a position at which we just exited a quoted string.
// At these positions, weird stuff happens. ;)
func bashUnescape(s string) (ss []string, weird bool) {
	var prevRuneMode, mode bashMode
	var escaped bool

	var result []string
	var cur []rune

	split := func() {
		result = append(result, string(cur))
		cur = cur[:0]
	}

	splitIfNotEmpty := func() {
		if len(cur) > 0 {
			split()
		}
	}

	var r rune
	for _, r = range s {
		prevRuneMode = bashmodeNormal
		switch mode {
		case bashmodeNormal:
			if escaped {
				cur = append(cur, r)
				escaped = false
			} else {
				switch r {
				case ' ':
					splitIfNotEmpty()
				case '\\':
					escaped = true
				case '"':
					prevRuneMode = mode
					mode = bashmodeDoubleQuote
					splitIfNotEmpty()
				case '\'':
					prevRuneMode = mode
					mode = bashmodeSingleQuote
					splitIfNotEmpty()
				default:
					cur = append(cur, r)
				}
			}
		case bashmodeDoubleQuote:
			if escaped {
				// we can only escape \ and " in doublequote mode
				switch r {
				case '\\', '"':
					cur = append(cur, r)
				default:
					cur = append(cur, '\\', r)
				}
				escaped = false
			} else {
				switch r {
				case '\\':
					escaped = true
				case '"':
					prevRuneMode = mode
					mode = bashmodeNormal
					split()
				default:
					cur = append(cur, r)
				}
			}
		case bashmodeSingleQuote:
			// escaping isn't possible in singlequote mode
			switch r {
			case '\'':
				prevRuneMode = mode
				mode = bashmodeNormal
				split()
			default:
				cur = append(cur, r)
			}
		}
	}

	split()

	return result, mode == bashmodeNormal && (prevRuneMode == bashmodeSingleQuote || prevRuneMode == bashmodeDoubleQuote)
}

func filterPrefix(pre string, ss ...string) []string {
	var result []string
	for _, s := range ss {
		if strings.HasPrefix(s, pre) {
			result = append(result, s)
		}
	}
	return result
}

func printlns(ss []string) {
	for _, s := range ss {
		fmt.Print(s)
		fmt.Println()
	}
}

func bashCompletion(args []string) {
	switch penultimate(args) {
	case "-c":
		printlns(conditional(args))
		return
	case "-d":
		// handled by wrapper bash script
		return
	case "-e":
		printlns(filterPrefix(last(args), "txt", "json", "csv", "influxdb"))
		return
	case "-f", "-l", "-h", "--help":
		return
	case "-i":
		printlns(ifaces(args))
		return
	case "-n":
		return
	case "-resolve-rows", "-resolve-timeout":
		return
	case "-s":
		printlns(filterPrefix(last(args), "bytes", "packets", "time"))
		return
	}

	switch {
	case strings.HasPrefix(last(args), "-"):
		printlns(flag(args))
		return
	default:
		printlns(queryType(args))
		return
	}
}

// Outputs a \n-separated list of possible bash-completions to stdout.
//
// compPoint: 1-based index indicating cursor position in compLine
//
// compLine: command line input, e.g. "goquery -i eth0 -c '"
func bash(compPoint int, compLine string) {
	// if the cursor points past the end of the line, something's wrong.
	if len(compLine) < compPoint {
		return
	}

	// truncate compLine up to cursor position
	compLine = compLine[:compPoint]

	splitLine, weird := bashUnescape(compLine)
	if len(splitLine) < 1 || weird {
		return
	}

	bashCompletion(splitLine)
}

func main() {
	defer func() {
		// We never want to confront the user with a huge panic message.
		if r := recover(); r != nil {
			os.Exit(1)
		}
	}()

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Please specify a completion mode.\n")
		return
	}

	switch os.Args[1] {
	case "bash":
		if len(os.Args) < 4 {
			return
		}

		compPoint, err := strconv.Atoi(os.Args[2])
		if err != nil {
			return
		}

		compLine := os.Args[3]

		bash(compPoint, compLine)
	case "-version":
		fmt.Printf("goquery_completion\n%s", version.Version())
	default:
		fmt.Fprintf(os.Stderr, "Unknown completion mode: %s Implemented modes: %s\n", os.Args[1], "bash, -version")
	}
}
