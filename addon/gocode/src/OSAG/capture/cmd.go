/////////////////////////////////////////////////////////////////////////////////
//
// cmd.go
//
// Written by Lorenz Breidenbach lob@open.ch, December 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	capconfig "OSAG/capture/config"
	"OSAG/goDB"
	"OSAG/goProbe"
	"OSAG/version"
)

const (
	// MAX_IFACES is the maximum number of interfaces we can monitor
	MAX_IFACES = 1024

	DB_WRITE_INTERVAL   = 300 // seconds
	CONTROL_SOCKET      = "control.sock"
	WRITEOUTSCHAN_DEPTH = 100

	// TODO(lob): For debugging. Consider removing this later.
	CONTROL_CMD_DEBUGSTATUS = "DEBUGSTATUS"

	CONTROL_CMD_STATUS = "STATUS"
	CONTROL_CMD_RELOAD = "RELOAD"
	CONTROL_CMD_ERRORS = "ERRORS"

	CONTROL_REPLY_DONE       = "DONE"
	CONTROL_REPLY_ERROR      = "ERROR"
	CONTROL_REPY_UNKNOWN_CMD = "UNKNOWN COMMAND"
)

// flag handling
var (
	flagConfigFile string
	flagVersion    bool
)

func init() {
	flag.StringVar(&flagConfigFile, "config", "", "path to configuration `file`")
	flag.BoolVar(&flagVersion, "version", false, "print version and exit")
}

// A writeout consists of a channel over which the individual
// interfaces' TaggedAggFlowMaps are sent and is tagged with
// the timestamp of when it was triggered.
type writeout struct {
	Chan      <-chan goProbe.TaggedAggFlowMap
	Timestamp time.Time
}

var (
	// cfg may be potentially accessed from multiple goroutines,
	// so we need to synchronize access.
	configMutex sync.Mutex
	config      *capconfig.Config

	// dbpath is set once in the beginning of executing goprobe
	// and then never changed
	dbpath string

	// captureManager and lastRotation may also be accessed
	// from multiple goroutines, so we need to synchronize access.
	captureManagerMutex sync.Mutex
	captureManager      *goProbe.CaptureManager
	lastRotation        time.Time
)

// reloadConfig attempts to reload the configuration file and updates
// the global config if successful.
func reloadConfig() error {
	c, err := capconfig.ParseFile(flagConfigFile)
	if err != nil {
		return fmt.Errorf("Failed to reload config file: %s", err)
	}

	if len(c.Interfaces) > MAX_IFACES {
		return fmt.Errorf("Cannot monitor more than %d interfaces.", MAX_IFACES)
	}

	if config != nil && dbpath != c.DBPath {
		return fmt.Errorf("Failed to reload config file: Cannot change database path while running.")
	}
	config = c
	return nil
}

func main() {
	// A general note on error handling: Any errors encountered during startup that make it
	// impossible to run are logged to stderr before the program terminates with a
	// non-zero exit code.
	// Issues encountered during capture will be logged to syslog.

	flag.Parse()
	if flagVersion {
		fmt.Printf("goProbe %s\n", version.VersionText())
		return
	}

	if flagConfigFile == "" {
		fmt.Fprintf(os.Stderr, "Please specify a config file.\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Initialize logger
	if err := goProbe.InitGPLog(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize Logger. Exiting!\n")
		os.Exit(1)
	}

	// Config file
	var err error
	config, err = capconfig.ParseFile(flagConfigFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, fmt.Sprintf("Failed to load config file: %s\n", err))
		os.Exit(1)
	}
	dbpath = config.DBPath
	goProbe.SysLog.Debug("Loaded config file")

	// It doesn't make sense to monitor zero interfaces
	if len(config.Interfaces) == 0 {
		fmt.Fprintf(os.Stderr, "No interfaces have been specified in the configuration file.\n")
		os.Exit(1)
	}
	// Limit the number of interfaces
	if len(config.Interfaces) > MAX_IFACES {
		fmt.Fprintf(os.Stderr, "Cannot monitor more than %d interfaces.\n", MAX_IFACES)
		os.Exit(1)
	}

	// We quit on encountering SIGTERM or SIGINT (see further down)
	sigExitChan := make(chan os.Signal, 1)
	signal.Notify(sigExitChan, syscall.SIGTERM, os.Interrupt)

	// Create DB directory if it doesn't exist already.
	if err := os.MkdirAll(dbpath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create database directory: '%s'\n", err)
		os.Exit(1)
	}

	// Open control socket
	listener, err := net.Listen("unix", filepath.Join(dbpath, CONTROL_SOCKET))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to listen on control socket '%s': %s\n", CONTROL_SOCKET, err)
		os.Exit(1)
	}
	defer listener.Close()

	// Initialize packet logger
	ifaces := make([]string, len(config.Interfaces))
	i := 0
	for k, _ := range config.Interfaces {
		ifaces[i] = k
		i++
	}
	goProbe.InitPacketLog(config.DBPath, ifaces)
	defer goProbe.PacketLog.Close()

	// None of the initialization steps failed.
	goProbe.SysLog.Info("Started goProbe")

	// Start goroutine for writeouts
	writeoutsChan := make(chan writeout, WRITEOUTSCHAN_DEPTH)
	completedWriteoutsChan := make(chan struct{})
	go handleWriteouts(writeoutsChan, completedWriteoutsChan, config.SyslogFlows)

	lastRotation = time.Now()

	captureManager = goProbe.NewCaptureManager()
	// No captures are being deleted here, so we can safely discard the channel we pass
	captureManagerMutex.Lock()
	captureManager.Update(config.Interfaces, make(chan goProbe.TaggedAggFlowMap))
	captureManagerMutex.Unlock()

	// We're ready to accept commands on the control socket
	go handleControlSocket(listener, writeoutsChan)

	// Start regular rotations
	go handleRotations(writeoutsChan)

	// Wait for signal to exit
	<-sigExitChan

	goProbe.SysLog.Debug("Shutting down")

	// We intentionally don't unlock the mutex hereafter,
	// because the program exits anyways. This ensures that there
	// can be no new Rotations/Updates/etc... while we're shutting down.
	captureManagerMutex.Lock()
	captureManager.DisableAll()

	// One last writeout
	woChan := make(chan goProbe.TaggedAggFlowMap, MAX_IFACES)
	writeoutsChan <- writeout{woChan, time.Now()}
	captureManager.RotateAll(woChan)
	close(woChan)
	close(writeoutsChan)

	captureManager.CloseAll()

	<-completedWriteoutsChan

	return
}

func handleRotations(writeoutsChan chan<- writeout) {
	// One rotation every DB_WRITE_INTERVAL seconds...
	ticker := time.NewTicker(time.Second * time.Duration(DB_WRITE_INTERVAL))
	for {
		select {
		case <-ticker.C:
			captureManagerMutex.Lock()
			goProbe.SysLog.Debug("Initiating flow data flush")

			lastRotation = time.Now()
			woChan := make(chan goProbe.TaggedAggFlowMap, MAX_IFACES)
			writeoutsChan <- writeout{woChan, lastRotation}
			captureManager.RotateAll(woChan)
			close(woChan)

			if len(writeoutsChan) > 2 {
				if len(writeoutsChan) > WRITEOUTSCHAN_DEPTH {
					goProbe.SysLog.Err(fmt.Sprintf("Writeouts are lagging behind too much: Queue length is %d", len(writeoutsChan)))
					os.Exit(1)
				}
				goProbe.SysLog.Warning(fmt.Sprintf("Writeouts are lagging behind: Queue length is %d", len(writeoutsChan)))
			}

			goProbe.SysLog.Debug("Restarting any interfaces that have encountered errors.")
			captureManager.EnableAll()
			captureManagerMutex.Unlock()
		}
	}
}

func handleWriteouts(writeoutsChan <-chan writeout, doneChan chan<- struct{}, logToSyslog bool) {
	writeoutsCount := 0
	dbWriters := make(map[string]*goDB.DBWriter)
	lastWrite := make(map[string]int)

	var syslogWriter *goDB.SyslogDBWriter
	if logToSyslog {
		var err error
		if syslogWriter, err = goDB.NewSyslogDBWriter(); err != nil {
			// we are not failing here due to the fact that a DB write out should still be attempted.
			// TODO: consider making a hard fail configurable
			goProbe.SysLog.Err(fmt.Sprintf("Failed to create syslog based flow writer: %s", err.Error()))
		}
	}

	for writeout := range writeoutsChan {
		t0 := time.Now()
		var summaryUpdates []goDB.InterfaceSummaryUpdate
		count := 0
		for taggedMap := range writeout.Chan {
			// Ensure that there is a DBWriter for the given interface
			_, exists := dbWriters[taggedMap.Iface]
			if !exists {
				w := goDB.NewDBWriter(dbpath, taggedMap.Iface)
				dbWriters[taggedMap.Iface] = w
			}

			// Prep metadata for current block
			meta := goDB.BlockMetadata{}
			meta.PcapPacketsReceived = -1
			meta.PcapPacketsDropped = -1
			meta.PcapPacketsIfDropped = -1
			if taggedMap.Stats.Pcap != nil {
				meta.PcapPacketsReceived = taggedMap.Stats.Pcap.PacketsReceived
				meta.PcapPacketsDropped = taggedMap.Stats.Pcap.PacketsDropped
				meta.PcapPacketsIfDropped = taggedMap.Stats.Pcap.PacketsIfDropped
			}
			meta.PacketsLogged = taggedMap.Stats.PacketsLogged
			meta.Timestamp = writeout.Timestamp.Unix()

			// Write to database, update summary
			update, err := dbWriters[taggedMap.Iface].Write(taggedMap.Map, meta, writeout.Timestamp.Unix())
			lastWrite[taggedMap.Iface] = writeoutsCount
			if err != nil {
				goProbe.SysLog.Err(fmt.Sprintf("Error during writeout: %s", err.Error()))
			} else {
				summaryUpdates = append(summaryUpdates, update)
			}

			// write out flows to syslog if necessary
			if logToSyslog {
				if syslogWriter != nil {
					syslogWriter.Write(taggedMap.Map, taggedMap.Iface, writeout.Timestamp.Unix())
				} else {
					goProbe.SysLog.Err("Cannot write flows to <nil> syslog writer. Attempting reinitialization.")

					// try to reinitialize the writer
					if syslogWriter, err = goDB.NewSyslogDBWriter(); err != nil {
						goProbe.SysLog.Err(fmt.Sprintf("Failed to reinitialize syslog writer: %s", err.Error()))
					}
				}
			}

			count++
		}

		// We are done with the writeout, let's try to write the updated summary
		err := goDB.ModifyDBSummary(dbpath, 10*time.Second, func(summ *goDB.DBSummary) (*goDB.DBSummary, error) {
			if summ == nil {
				summ = goDB.NewDBSummary()
			}
			for _, update := range summaryUpdates {
				summ.Update(update)
			}
			return summ, nil
		})
		if err != nil {
			goProbe.SysLog.Err(fmt.Sprintf("Error updating summary: %s", err.Error()))
		}

		// Clean up dead writers. We say that a writer is dead
		// if it hasn't been used in the last few writeouts.
		var remove []string
		for iface, last := range lastWrite {
			if writeoutsCount-last >= 3 {
				remove = append(remove, iface)
			}
		}
		for _, iface := range remove {
			delete(dbWriters, iface)
			delete(lastWrite, iface)
		}

		writeoutsCount++
		goProbe.SysLog.Debug(fmt.Sprintf("Completed writeout (count: %d) in %s", count, time.Now().Sub(t0)))
	}

	goProbe.SysLog.Debug("Completed all writeouts")
	doneChan <- struct{}{}
}

// handleControlSocket accepts connections on the given listener and handles any interactions
// with clients.
//
// There is no mechanism for graceful termination because we don't need one:
// We never stop listening on the control socket once we have started until the program
// terminates anyways and/or listener.Accept() fails.
func handleControlSocket(listener net.Listener, writeoutsChan chan<- writeout) {
	for {
		conn, err := listener.Accept()

		if err != nil {
			goProbe.SysLog.Info(fmt.Sprintf("Stopped listening on control socket because: %s.", err))
			return
		}
		goProbe.SysLog.Debug(fmt.Sprintf("Accepted connection on control socket."))

		// handle connection
		go func(conn net.Conn) {
			defer conn.Close()

			var writeError error
			writeLn := func(msg string) {
				if writeError != nil {
					return
				}
				_, writeError = io.WriteString(conn, msg+"\n")
			}

			scanner := bufio.NewScanner(conn)
			for scanner.Scan() {
				switch scanner.Text() {
				case CONTROL_CMD_RELOAD:
					configMutex.Lock()
					if err := reloadConfig(); err == nil {
						captureManagerMutex.Lock()
						woChan := make(chan goProbe.TaggedAggFlowMap, MAX_IFACES)
						writeoutsChan <- writeout{woChan, time.Now()}
						captureManager.Update(config.Interfaces, woChan)
						close(woChan)
						captureManagerMutex.Unlock()

						writeLn(CONTROL_REPLY_DONE)
					} else {
						goProbe.SysLog.Err(err.Error())
						writeLn(CONTROL_REPLY_ERROR)
					}
					configMutex.Unlock()
				case CONTROL_CMD_STATUS, CONTROL_CMD_DEBUGSTATUS:
					captureManagerMutex.Lock()
					writeLn(fmt.Sprintf("%.0f", time.Now().Sub(lastRotation).Seconds()))
					for iface, status := range captureManager.StatusAll() {
						var stateStr string
						switch scanner.Text() {
						case CONTROL_CMD_STATUS:
							stateStr = stateMessage(status.State)
						case CONTROL_CMD_DEBUGSTATUS:
							stateStr = status.State.String()
						}
						if status.Stats.Pcap == nil {
							writeLn(fmt.Sprintf("%s %s %d NA NA NA",
								iface,
								stateStr,
								status.Stats.PacketsLogged,
							))
						} else {
							writeLn(fmt.Sprintf("%s %s %d %d %d %d",
								iface,
								stateStr,
								status.Stats.PacketsLogged,
								status.Stats.Pcap.PacketsReceived,
								status.Stats.Pcap.PacketsDropped,
								status.Stats.Pcap.PacketsIfDropped,
							))
						}
					}
					captureManagerMutex.Unlock()

					writeLn(CONTROL_REPLY_DONE)
				case CONTROL_CMD_ERRORS:
					captureManagerMutex.Lock()
					for iface, errs := range captureManager.ErrorsAll() {
						if len(errs) > 0 {
							writeLn(fmt.Sprintf("%s:", iface))
							for errString, count := range errs {
								writeLn(fmt.Sprintf(" [%8d] %s", count, errString))
							}
						} else {
							writeLn(fmt.Sprintf("%s:\n no errors", iface))
						}
					}

					captureManagerMutex.Unlock()

					writeLn(CONTROL_REPLY_DONE)
				default:
					writeLn(CONTROL_REPY_UNKNOWN_CMD)
				}
			}
			if writeError != nil {
				goProbe.SysLog.Debug(fmt.Sprintf("Error on control socket: %s", writeError))
				return
			}
			if scanner.Err() != nil {
				goProbe.SysLog.Debug(fmt.Sprintf("Error on control socket: %s", scanner.Err()))
				return
			}
		}(conn)
	}
}

// Returns a brief string without whitespace
// that represents the argument CaptureState.
func stateMessage(cs goProbe.CaptureState) string {
	switch cs {
	case goProbe.CAPTURE_STATE_UNINITIALIZED:
		return "inactive"
	case goProbe.CAPTURE_STATE_INITIALIZED:
		return "inactive"
	case goProbe.CAPTURE_STATE_ACTIVE:
		return "active"
	case goProbe.CAPTURE_STATE_ERROR:
		return "inactive"
	default:
		return "unknown"
	}
}
