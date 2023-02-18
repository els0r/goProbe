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
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/els0r/goProbe/pkg/logging"
	"github.com/els0r/goProbe/pkg/types/hashmap"
)

const (
	// MaxIfaces is the maximum number of interfaces we can monitor
	MaxIfaces = 1024

	// WriteoutsChanDepth sets the maximum amount of writeouts that can be queued
	WriteoutsChanDepth = 100
)

// TaggedAggFlowMap represents an aggregated
// flow map tagged with Stats and an
// an interface name.
//
// Used by Manager to return the results of
// RotateAll() and Update().
type TaggedAggFlowMap struct {
	Map   *hashmap.AggFlowMap
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

// WriteoutHandler provides the writeout and completion channels for external callers
type WriteoutHandler struct {
	DoneWriting  context.CancelFunc
	WriteoutChan chan Writeout
}

// NewWriteoutHandler prepares a new handler for initiating flow writeouts
func NewWriteoutHandler() *WriteoutHandler {
	return &WriteoutHandler{
		WriteoutChan: make(chan Writeout, WriteoutsChanDepth),
	}
}

// Manager manages a set of Capture instances.
// Each interface can be associated with up to one Capture.
type Manager struct {
	sync.Mutex
	captures        map[string]*ManagedCapture
	LastRotation    time.Time
	WriteoutHandler *WriteoutHandler
	ctx             context.Context
}

type ManagedCapture struct {
	capture *Capture
	cancel  context.CancelFunc
}

// NewManager creates a new Manager
func NewManager(ctx context.Context) *Manager {
	return &Manager{
		captures:        make(map[string]*ManagedCapture),
		LastRotation:    time.Now(),
		WriteoutHandler: NewWriteoutHandler(),
		ctx:             ctx,
	}
}

func (cm *Manager) ifaceNames() []string {
	ifaces := make([]string, 0, len(cm.captures))

	cm.Lock()
	for iface := range cm.captures {
		ifaces = append(ifaces, iface)
	}
	cm.Unlock()

	return ifaces
}

func (cm *Manager) enable(ifaces map[string]config.CaptureConfig) {
	var rg RunGroup

	for iface, config := range ifaces {
		if cm.captureExists(iface) {
			mc, config := cm.getCapture(iface), config
			rg.Run(func() {
				mc.capture.Update(config)
			})
		} else {
			capCtx, cancel := context.WithCancel(cm.ctx)

			capture := NewCapture(capCtx, iface, config)

			cm.setCapture(iface, &ManagedCapture{capture: capture, cancel: cancel})

			logger := logging.WithContext(capture.ctx)
			logger.Info(fmt.Sprintf("added interface to capture list"))

			capture.Run()
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
	logger := logging.Logger()

	t0 := time.Now()

	var rg RunGroup
	for _, mc := range cm.capturesCopy() {
		mc := mc
		rg.Run(func() {
			mc.capture.Run()
		})
	}
	rg.Wait()

	elapsed := time.Since(t0).Round(time.Millisecond)

	logger.With("elapsed", elapsed.String()).Debugf("completed interface capture check")
}

func (cm *Manager) getCapture(iface string) *ManagedCapture {
	cm.Lock()
	c := cm.captures[iface]
	cm.Unlock()

	return c
}

func (cm *Manager) setCapture(iface string, capture *ManagedCapture) {
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

func (cm *Manager) capturesCopy() map[string]*ManagedCapture {
	copyMap := make(map[string]*ManagedCapture)

	cm.Lock()
	for iface, capture := range cm.captures {
		copyMap[iface] = capture
	}
	cm.Unlock()

	return copyMap
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
func (cm *Manager) Update(ifaces map[string]config.CaptureConfig, returnChan chan TaggedAggFlowMap) {
	logger := logging.Logger()

	t0 := time.Now()

	ifaceSet := make(map[string]struct{})
	for iface := range ifaces {
		ifaceSet[iface] = struct{}{}
	}

	// Contains the names of all interfaces we are shutting down and deleting.
	var disableIfaces []string

	cm.Lock()
	for iface := range cm.captures {
		if _, exists := ifaceSet[iface]; !exists {
			disableIfaces = append(disableIfaces, iface)
		}
	}
	cm.Unlock()

	var rg RunGroup
	rg.Run(func() {
		cm.enable(ifaces)
	})
	rg.Wait()

	for _, iface := range disableIfaces {
		iface, mc := iface, cm.getCapture(iface)
		rg.Run(func() {
			aggFlowMap, stats := mc.capture.Rotate()
			returnChan <- TaggedAggFlowMap{
				aggFlowMap,
				stats,
				iface,
			}

		})

		// close capture and delete from list of managed captures
		logger = logging.WithContext(mc.capture.ctx)

		mc.cancel()
		cm.delCapture(iface)

		logger.Info(fmt.Sprintf("deleted interface from capture list"))
	}
	rg.Wait()

	elapsed := time.Since(t0).Round(time.Millisecond)

	logger.With("elapsed", elapsed).Debug("updated interface list")
}

// StatusAll returns the statuses of all managed Capture instances.
func (cm *Manager) StatusAll() map[string]Status {
	statusmapMutex := sync.Mutex{}
	statusmap := make(map[string]Status)

	var rg RunGroup
	for iface, mc := range cm.capturesCopy() {
		iface, mc := iface, mc
		rg.Run(func() {
			status := mc.capture.Status()
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

	for i, mc := range cm.capturesCopy() {
		i, mc := i, mc

		if iface == i || iface == "all" {
			rg.Run(func() {
				f := mc.capture.Flows()
				ifaceFlowsMutex.Lock()
				ifaceFlows[i] = f
				ifaceFlowsMutex.Unlock()
			})
		}
	}
	rg.Wait()

	if len(ifaceFlows) == 0 {
		return nil, fmt.Errorf("no active flows found for interface %q", iface)
	}
	return ifaceFlows, nil
}

// ErrorsAll returns the error maps of all managed Capture instances.
func (cm *Manager) ErrorsAll() map[string]ErrorMap {
	errmapMutex := sync.Mutex{}
	errormap := make(map[string]ErrorMap)

	var rg RunGroup
	for iface, mc := range cm.capturesCopy() {
		iface, mc := iface, mc
		rg.Run(func() {
			errs := mc.capture.Errors()
			errmapMutex.Lock()
			errormap[iface] = errs
			errmapMutex.Unlock()
		})
	}
	rg.Wait()

	return errormap
}

// RotateAll returns the state of all managed Capture instances.
//
// The resulting TaggedAggFlowMaps will be sent over returnChan and
// be tagged with the given timestamp.
func (cm *Manager) RotateAll(returnChan chan TaggedAggFlowMap) {
	logger := logging.Logger()

	t0 := time.Now()

	var rg RunGroup

	for iface, mc := range cm.capturesCopy() {
		iface, mc := iface, mc
		rg.Run(func() {
			aggFlowMap, stats := mc.capture.Rotate()
			returnChan <- TaggedAggFlowMap{
				aggFlowMap,
				stats,
				iface,
			}
		})
	}
	rg.Wait()

	elapsed := time.Since(t0).Round(time.Millisecond)

	logger.With("elapsed", elapsed).Debug("completed rotation of all captures")
}

// CloseAll closes and deletes all Capture instances managed by the
// Manager
func (cm *Manager) CloseAll() {
	logger := logging.Logger()

	t0 := time.Now()

	var rg RunGroup

	for _, mc := range cm.capturesCopy() {
		mc := mc
		rg.Run(func() {
			mc.cancel()
		})
	}

	cm.Lock()
	cm.captures = make(map[string]*ManagedCapture)
	cm.Unlock()

	rg.Wait()

	elapsed := time.Since(t0).Round(time.Millisecond)

	logger.With("elapsed", elapsed).Debug("closed all captures")
}
