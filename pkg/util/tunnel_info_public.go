/////////////////////////////////////////////////////////////////////////////////
//
// tunnel_info_public.go
//
// Written by Lorenz Breidenbach lob@open.ch, February 2016
// Copyright (c) 2016 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

// +build !OSAG

package util

// TunnelInfos is a no-op for the public release.
func TunnelInfos() map[string]TunnelInfo {
	return nil
}
