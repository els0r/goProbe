/////////////////////////////////////////////////////////////////////////////////
//
// tunnel_info.go
//
// Written by Lorenz Breidenbach lob@open.ch, February 2016
// Copyright (c) 2016 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

// Package util is used to store info about the physical interfaces of IPSEC tunnels. The mapping is specific to an environment that has multiple IPSec tunnels set up.
package util

// TunnelInfo stores information about the physical interfaces of IPSec tunnels
type TunnelInfo struct {
	PhysicalIface string
	Peer          string
}
