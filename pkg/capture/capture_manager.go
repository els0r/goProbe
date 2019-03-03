/////////////////////////////////////////////////////////////////////////////////
//
// capture_manager.go
//
// Written by Lorenz Breidenbach lob@open.ch,
//            Lennart Elsen lel@open.ch, December 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package capture

import (
	"fmt"
	"sync"
	"time"

	"github.com/els0r/goProbe/pkg/goDB"
	"github.com/els0r/log"
)

const (
	// MAX_IFACES is the maximum number of interfaces we can monitor
	MAX_IFACES = 1024

	WRITEOUTSCHAN_DEPTH = 100
)

// TaggedAggFlowMap represents an aggregated
// flow map tagged with Stats and an
// an interface name.
//
// Used by Manager to return the results of
// RotateAll() and Update().
type TaggedAggFlowMap struct {
	Map   goDB.AggFlowMap
	Stats Stats  `json:"stats,omitempty"`
	Iface string `json:"iface"`
}

// Writeout consists of a channel over which the individual
// interfaces' TaggedAggFlowMaps are sent and is tagged with
// the timestamp of when it was triggered.
type Writeout struct {
	Chan      <-chan TaggedAggFlowMap
	Timestamp time.Time
}

type WriteoutHandler struct {
	CompletedChan chan struct{}
	WriteoutChan  chan Writeout
}

// NewWriteoutHandler prepares a new handler for initiating flow writeouts
func NewWriteoutHandler() *WriteoutHandler {
	return &WriteoutHandler{
		CompletedChan: make(chan struct{}),
		WriteoutChan:  make(chan Writeout, WRITEOUTSCHAN_DEPTH),
	}
}

// Manager manages a set of Capture instances.
// Each interface can be associated with up to one Capture.
type Manager struct {
	sync.Mutex
	captures        map[string]*Capture
	logger          log.Logger
	LastRotation    time.Time
	WriteoutHandler *WriteoutHandler
}

// NewManager creates a new Manager and
// returns a pointer to it.
func NewManager(logger log.Logger) *Manager {
	return &Manager{
		captures:        make(map[string]*Capture),
		logger:          logger,
		LastRotation:    time.Now(),
		WriteoutHandler: NewWriteoutHandler(),
	}
}

func (cm *Manager) ifaceNames() []string {
	ifaces := make([]string, 0, len(cm.captures))

	cm.Lock()
	for iface, _ := range cm.captures {
		ifaces = append(ifaces, iface)
	}
	cm.Unlock()

	return ifaces
}

func (cm *Manager) enable(ifaces map[string]Config) {
	var rg RunGroup

	for iface, config := range ifaces {
		if cm.captureExists(iface) {
			capture, config := cm.getCapture(iface), config
			rg.Run(func() {
				capture.Update(config)
			})
		} else {
			capture := NewCapture(iface, config, cm.logger)
			cm.setCapture(iface, capture)

			cm.logger.Info(fmt.Sprintf("Added interface '%s' to capture list.", iface))

			rg.Run(func() {
				capture.Enable()
			})
		}
	}

	rg.Wait()
}

// EnableAll attempts to enable all managed Capture instances.
//
// Returns once all instances have been enabled.
// Note that each attempt may fail, for example if the interface
// that a Capture is supposed to monitor ceases to exist. Use
// StateAll() to find out wheter the Capture instances encountered
// an error.
func (cm *Manager) EnableAll() {
	t0 := time.Now()

	var rg RunGroup

	for _, capture := range cm.capturesCopy() {
		capture := capture
		rg.Run(func() {
			capture.Enable()
		})
	}

	rg.Wait()

	cm.logger.Debug(fmt.Sprintf("Completed interface capture check in %s", time.Now().Sub(t0)))
}

func (cm *Manager) disable(ifaces []string) {
	var rg RunGroup

	for _, iface := range ifaces {
		iface := iface
		rg.Run(func() {
			cm.getCapture(iface).Disable()
		})
	}
	rg.Wait()
}

func (cm *Manager) getCapture(iface string) *Capture {
	cm.Lock()
	c := cm.captures[iface]
	cm.Unlock()

	return c
}

func (cm *Manager) setCapture(iface string, capture *Capture) {
	cm.Lock()
	cm.captures[iface] = capture
	cm.Unlock()
}

func (cm *Manager) delCapture(iface string) {
	cm.Lock()
	delete(cm.captures, iface)
	cm.Unlock()
}

func (cm *Manager) captureExists(iface string) bool {
	cm.Lock()
	_, exists := cm.captures[iface]
	cm.Unlock()

	return exists
}

func (cm *Manager) capturesCopy() map[string]*Capture {
	copyMap := make(map[string]*Capture)

	cm.Lock()
	for iface, capture := range cm.captures {
		copyMap[iface] = capture
	}
	cm.Unlock()

	return copyMap
}

// DisableAll disables all managed Capture instances.
//
// Returns once all instances have been disabled.
// The instances are not deleted, so you may later enable them again;
// for example, by calling EnableAll().
func (cm *Manager) DisableAll() {
	t0 := time.Now()

	cm.disable(cm.ifaceNames())

	cm.logger.Info(fmt.Sprintf("Disabled all captures in %s", time.Now().Sub(t0)))
}

// Update attempts to enable all Capture instances given by
// ifaces. If an instance doesn't exist, it will be created.
// If an instance has encountered an error or an instance's configuration
// differs from the one specified in ifaces, it will be re-enabled.
// Finally, if the Manager manages an instance for an iface that does
// not occur in ifaces, the following actions are performed on the instance:
// (1) the instance will be disabled,
// (2) the instance will be rotated,
// (3) the resulting flow data will be sent over returnChan,
// (tagged with the interface name and stats),
// (4) the instance will be closed,
// and (5) the instance will be completely removed from the Manager.
//
// Returns once all the above actions have been completed.
func (cm *Manager) Update(ifaces map[string]Config, returnChan chan TaggedAggFlowMap) {
	t0 := time.Now()

	ifaceSet := make(map[string]struct{})
	for iface := range ifaces {
		ifaceSet[iface] = struct{}{}
	}

	// Contains the names of all interfaces we are shutting down and deleting.
	var disableIfaces []string

	cm.Lock()
	for iface, _ := range cm.captures {
		if _, exists := ifaceSet[iface]; !exists {
			disableIfaces = append(disableIfaces, iface)
		}
	}
	cm.Unlock()

	var rg RunGroup
	// disableIfaces and ifaces are disjunct, so we can run these in parallel.
	rg.Run(func() {
		cm.disable(disableIfaces)
	})
	rg.Run(func() {
		cm.enable(ifaces)
	})
	rg.Wait()

	for _, iface := range disableIfaces {
		iface, capture := iface, cm.getCapture(iface)
		rg.Run(func() {
			aggFlowMap, stats := capture.Rotate()
			returnChan <- TaggedAggFlowMap{
				aggFlowMap,
				stats,
				iface,
			}

			capture.Close()
		})

		cm.delCapture(iface)
		cm.logger.Info(fmt.Sprintf("Deleted interface '%s' from capture list.", iface))
	}
	rg.Wait()

	cm.logger.Debug(fmt.Sprintf("Updated interface list in %s", time.Now().Sub(t0)))
}

// StatusAll() returns the statuses of all managed Capture instances.
func (cm *Manager) StatusAll() map[string]Status {
	statusmapMutex := sync.Mutex{}
	statusmap := make(map[string]Status)

	var rg RunGroup
	for iface, capture := range cm.capturesCopy() {
		iface, capture := iface, capture
		rg.Run(func() {
			status := capture.Status()
			statusmapMutex.Lock()
			statusmap[iface] = status
			statusmapMutex.Unlock()
		})
	}
	rg.Wait()

	return statusmap
}

// ActiveFlows returns a copy of the current in-memory flow map. If iface is "all", flows for every interface are returned
func (cm *Manager) ActiveFlows(iface string) (map[string]*FlowLog, error) {
	var rg RunGroup

	ifaceFlows := make(map[string]*FlowLog)
	ifaceFlowsMutex := sync.Mutex{}

	for i, capture := range cm.capturesCopy() {
		i, capture := i, capture

		if iface == i || iface == "all" {
			rg.Run(func() {
				f := capture.Flows()
				ifaceFlowsMutex.Lock()
				ifaceFlows[i] = f
				ifaceFlowsMutex.Unlock()
			})
		}
	}
	rg.Wait()

	if len(ifaceFlows) == 0 {
		return nil, fmt.Errorf("no active flows found for interface \"%s\"", iface)
	}
	return ifaceFlows, nil
}

// ErrorsAll() returns the error maps of all managed Capture instances.
func (cm *Manager) ErrorsAll() map[string]errorMap {
	errmapMutex := sync.Mutex{}
	errormap := make(map[string]errorMap)

	var rg RunGroup
	for iface, capture := range cm.capturesCopy() {
		iface, capture := iface, capture
		rg.Run(func() {
			errs := capture.Errors()
			errmapMutex.Lock()
			errormap[iface] = errs
			errmapMutex.Unlock()
		})
	}
	rg.Wait()

	return errormap
}

// RotateAll() returns the state of all managed Capture instances.
//
// The resulting TaggedAggFlowMaps will be sent over returnChan and
// be tagged with the given timestamp.
func (cm *Manager) RotateAll(returnChan chan TaggedAggFlowMap) {
	t0 := time.Now()

	var rg RunGroup

	for iface, capture := range cm.capturesCopy() {
		iface, capture := iface, capture
		rg.Run(func() {
			aggFlowMap, stats := capture.Rotate()
			returnChan <- TaggedAggFlowMap{
				aggFlowMap,
				stats,
				iface,
			}
		})
	}
	rg.Wait()

	cm.logger.Debug(fmt.Sprintf("Completed rotation of all captures in %s", time.Now().Sub(t0)))
}

// CloseAll() closes and deletes all Capture instances managed by the
// Manager
func (cm *Manager) CloseAll() {
	var rg RunGroup

	for _, capture := range cm.capturesCopy() {
		capture := capture
		rg.Run(func() {
			capture.Close()
		})
	}

	cm.Lock()
	cm.captures = make(map[string]*Capture)
	cm.Unlock()

	rg.Wait()
}
