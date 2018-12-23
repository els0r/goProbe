/////////////////////////////////////////////////////////////////////////////////
//
// dbdir_public.go
//
// Written by Lennart Elsen lel@open.ch, February 2016
// Copyright (c) 2016 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

// +build !OSAG

package main

// set this variable on compile time
var goprobeConfigPath string

type gpConfig struct {
	DBPath     string                 `json:"db_path"`
	Interfaces map[string]interface{} `json:"interfaces"`
}

func getDefaultDBDir() (string, error) {
	return "/opt/ntm/goProbe/db", nil
}
