/////////////////////////////////////////////////////////////////////////////////
//
// SyslogDBWriter.go
//
// Logging facility for dumping the raw flow information to syslog.
//
// Written by Lennart Elsen lel@open.ch, June 2016
// Copyright (c) 2016 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package goDB

import (
    "fmt"
    "log/syslog"
)

type SyslogDBWriter struct {
    logger *syslog.Writer
}

func NewSyslogDBWriter() (*SyslogDBWriter, error) {
    s := &SyslogDBWriter{}

    var err error
    if s.logger, err = syslog.Dial("unix", SOCKET_PATH, syslog.LOG_NOTICE, "ntm"); err != nil {
        return nil, err
    }
    return s, nil
}

func (s *SyslogDBWriter) Write(flowmap AggFlowMap, iface string, timestamp int64) {
    for flowKey, flowVal := range flowmap {
        s.logger.Info(
            fmt.Sprintf("%d,%s,%s,%s",
                timestamp,
                iface,
                flowKey.String(),
                flowVal.String(),
            ),
        )
    }
}
