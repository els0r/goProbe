/////////////////////////////////////////////////////////////////////////////////
//
// dns_test.go
//
// Written by Lorenz Breidenbach lob@open.ch, August 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package dns

import (
	"context"
	"os"
	"testing"
	"time"
)

func skipInCI(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skipf("skipping DNS tests in CI/CD mode")
	}
}

func TestLookup(t *testing.T) {
	skipInCI(t)
	t.Parallel()

	// 8.8.8.8 is google's DNS server. This lookup should yield the same
	// result for many years.
	ips2domains := TimedReverseLookup(context.Background(), []string{"8.8.8.8", "0.0.0.0"}, 2*time.Second)
	if domain, ok := ips2domains["8.8.8.8"]; ok && domain != "dns.google." {
		t.Fatalf("RDNS lookup yielded wrong result: %s", domain)
	} else if !ok {
		t.Log("RDNS lookup yielded no result. Perhaps your internet is down?")
		t.Skip()
	}

	if _, ok := ips2domains["0.0.0.0"]; ok {
		t.Fatalf("RDNS unexpectedly succeeded on 0.0.0.0.")
	}
}

func TestTimeout(t *testing.T) {
	skipInCI(t)
	t.Parallel()

	t0 := time.Now()
	_ = TimedReverseLookup(context.Background(), []string{"8.8.8.8", "8.8.4.4", "192.168.0.1", "10.0.0.1", "129.3.4.5"}, 1*time.Millisecond)
	t1 := time.Now()
	if t1.Sub(t0) > 10*time.Millisecond {
		t.Fatal("Timeout failed")
	}
}
