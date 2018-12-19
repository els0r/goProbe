/////////////////////////////////////////////////////////////////////////////////
//
// DBLog.go
//
// Log.ing Interface that all other interfaces get access to in order to write
// error messages to the underlying system logging facilities
//
// Written by Lennart Elsen lel@open.ch, July 2014
// Copyright (c) 2014 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package goDB

import (
	"log/syslog"
)

type DBLog struct {
	Log *syslog.Writer
}

var SysLog *syslog.Writer

const SLOG_ADDR = "127.0.0.1"
const SLOG_PORT = "514"

func InitDBLog() error {

	var err error
	if SysLog, err = syslog.Dial("udp", SLOG_ADDR+":"+SLOG_PORT, syslog.LOG_NOTICE, "goDB"); err != nil {
		return err
	}
	return nil
}
