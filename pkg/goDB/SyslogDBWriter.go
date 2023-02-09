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

	"github.com/els0r/goProbe/pkg/types/hashmap"
)

// SyslogDBWriter can write goProbe's flow map to a syslog destination
type SyslogDBWriter struct {
	logger *syslog.Writer
}

// NewSyslogDBWriter establishes a syslog connection and returns the flow writer
func NewSyslogDBWriter() (*SyslogDBWriter, error) {
	s := &SyslogDBWriter{}

	var err error
	if s.logger, err = syslog.Dial("unix", socketPath, syslog.LOG_NOTICE, "ntm"); err != nil {
		return nil, err
	}
	return s, nil
}

// Write writes the aggregated flows to the syslog writer
func (s *SyslogDBWriter) Write(flowmap *hashmap.AggFlowMap, iface string, timestamp int64) {
	for i := flowmap.Iter(); i.Next(); {
		s.logger.Info(
			fmt.Sprintf("%d,%s,%s,%s",
				timestamp,
				iface,
				i.Key(),
				i.Val().String(),
			),
		)
	}
}
