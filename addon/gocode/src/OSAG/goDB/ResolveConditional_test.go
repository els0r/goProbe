/////////////////////////////////////////////////////////////////////////////////
//
// ResolveConditional_test.go
//
// Written by Lorenz Breidenbach lob@open.ch, February 2016
// Copyright (c) 2016 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package goDB

import (
    "fmt"
    "testing"
    "time"
)

var resolveTests = []struct {
    conditional string
    timeout     time.Duration
    output      string
    success     bool
}{
    // doesn't exist and likely never will
    {
        "sip = 1b5ec0b902e689122ededcfac01ee69ed3c8422c.open.ch",
        2 * time.Second,
        "",
        false,
    },
    // super short timeout
    {
        "sip = google-public-dns-a.google.com | dip = google-public-dns-a.google.com", // Google's 8.8.8.8 DNS server
        1 * time.Nanosecond,
        "",
        false,
    },
    // should work (Google's 8.8.8.8 DNS server)
    {
        "sip = google-public-dns-a.google.com | dip = google-public-dns-a.google.com", //
        2 * time.Second,
        "((sip = 8.8.8.8 | sip = 2001:4860:4860::8888) | (dip = 8.8.8.8 | dip = 2001:4860:4860::8888))",
        true,
    },
    // do we leave non-sip and non-dip attributes untouched?
    {
        "((sip = 8.8.8.8 | l7proto = 10) | (dport = 80 | snet = 192.168.1.1/20))",
        2 * time.Second,
        "((sip = 8.8.8.8 | l7proto = 10) | (dport = 80 | snet = 192.168.1.1/20))",
        true,
    },
    // wrong domains
    {
        "sip = ..",
        2 * time.Second,
        "",
        false,
    },
    {
        "dip = .wtf",
        2 * time.Second,
        "",
        false,
    },
}

// Note that this test is inherently brittle since it relies on:
// * working DNS resolution
// * google-public-dns-a.google.com resolving to 2001:4860:4860::8888 and 8.8.8.8
//   I couldn't think of a more stable domain-IP pair, but of course google can
//   change this at any moment.
//
// It's probably still better to have a slightly brittle test than to have no test.
func TestResolveInConditional(t *testing.T) {
    for _, test := range resolveTests {
        tokens, err := TokenizeConditional(test.conditional)
        if err != nil {
            t.Fatalf("Tokenizing %v unexpectly failed. Error:\n%v", test.conditional, err)
        }
        node, err := parseConditional(tokens)
        if err != nil {
            t.Fatalf("Parsing %v unexpectly failed. Error:\n%v", tokens, err)
        }

        resolvedNode, err := resolve(node, test.timeout)
        if !test.success {
            if err == nil {
                fmt.Println(resolvedNode)
                t.Errorf("Expected to fail on input %v but didn't.",
                    test.conditional)
            }
        } else {
            if err != nil {
                t.Errorf("Unexpectedly failed on input %v. The error is: %s",
                    test.conditional, err)
            }
            if resolvedNode.String() != test.output {
                t.Errorf("Expected output: %s. Actual output: %s",
                    test.output, resolvedNode)
            }
        }
    }
}
