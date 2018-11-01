/////////////////////////////////////////////////////////////////////////////////
//
// GPLog.go
//
// Logging Interface that all other interfaces get access to in order to write
// error messages to the underlying system logging facilities
//
// Written by Lennart Elsen lel@open.ch, May 2014
// Copyright (c) 2014 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package goProbe

import (
    "fmt"
    "log/syslog"
    "os"
    "sync"

    "github.com/google/gopacket"
    "github.com/google/gopacket/layers"
    "github.com/google/gopacket/pcapgo"
)

type PacketLogWriter struct {
    sync.Mutex
    path    string
    writers map[string]*PcapWriter
}

type PcapWriter struct {
    file       *os.File
    pcapWriter *pcapgo.Writer
}

var SysLog *syslog.Writer
var PacketLog *PacketLogWriter

const (
    SLOG_ADDR = "127.0.0.1"
    SLOG_PORT = "514"
)

func InitGPLog() error {

    var err error
    if SysLog, err = syslog.Dial("udp", SLOG_ADDR+":"+SLOG_PORT, syslog.LOG_NOTICE, "goProbe"); err != nil {
        return err
    }
    return nil
}

func InitPacketLog(dbpath string, ifaces []string) {

    PacketLog = &PacketLogWriter{writers: make(map[string]*PcapWriter)}
    PacketLog.path = dbpath

    PacketLog.Lock()
    defer PacketLog.Unlock()

    for _, iface := range ifaces {
        PacketLog.writers[iface] = nil
    }
}

func (p *PacketLogWriter) Close() {
    for _, w := range p.writers {
        if w != nil {
            if w.file != nil {
                w.file.Close()
            }
        }
    }
}

func (p *PacketLogWriter) Log(iface string, packet gopacket.Packet, snapshotLen int) error {
    p.Lock()
    defer p.Unlock()

    var err error

    // create a new packet logger if nothing has been logged yet
    if p.writers[iface] == nil {
        pw := new(PcapWriter)

        // make sure the directory exists before logging the packet to disk. If this is the very first
        // time that goProbe is started, this is important
        if err = os.MkdirAll(p.path+"/"+iface, 0755); err != nil {
            return err
        }

        if pw.file, err = os.Create(p.path + "/" + iface + "/" + iface + "_errors.pcap"); err != nil {
            return err
        }
        pw.pcapWriter = pcapgo.NewWriter(pw.file)
        pw.pcapWriter.WriteFileHeader(uint32(snapshotLen), layers.LinkTypeEthernet)

        p.writers[iface] = pw
    }

    // log the packet
    if p.writers[iface].pcapWriter != nil && p.writers[iface].file != nil {
        if err = p.writers[iface].pcapWriter.WritePacket(packet.Metadata().CaptureInfo, packet.Data()); err != nil {
            return err
        }
    } else {
        return fmt.Errorf("packet log writer is nil")
    }
    return nil
}
