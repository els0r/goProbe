/////////////////////////////////////////////////////////////////////////////////
//
// dns_public.go
//
// Written by Lorenz Breidenbach lob@open.ch, October 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

// +build !OSAG

package dns

// CheckDNS is a no-op to check if a DNS resolver is present (deployment enviornment specific)
func CheckDNS() error {
	return nil
}
