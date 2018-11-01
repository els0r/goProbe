/////////////////////////////////////////////////////////////////////////////////
//
// config.go
//
// Type definitions and helper functions used throughout this package
//
// Written by Lennart Elsen      lel@open.ch and
//            Lorenz Breidenbach lob@open.ch, October 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package main

import "time"

// Config encapsulates all command line options one can pass to goQuery
type Config struct {
    QueryType      string
    Ifaces         string
    Conditions     string
    NumResults     int
    Help           bool
    HelpAdmin      bool
    Version        bool
    WipeAdmin      bool
    CleanAdmin     int64
    External       bool
    Sort           string
    SortAscending  bool
    Incoming       bool
    Outgoing       bool
    Sum            bool
    First          string
    Last           string
    BaseDir        string
    ListDB         bool
    Format         string
    MaxMemPercent  int
    Resolve        bool
    ResolveRows    int
    ResolveTimeout time.Duration
    ShowMgmtTraffic bool
}
