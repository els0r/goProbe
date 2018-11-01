/////////////////////////////////////////////////////////////////////////////////
//
// syslogConsts_public.go
//
// Constants for location of syslog socket file
//
// Written by Lennart Elsen lel@open.ch, June 2016
// Copyright (c) 2016 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

// +build !OSAG

package goDB

const (
    SOCKET_PATH = "/var/run/goprobe.sock"
)
