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
	"sort"
	"sync"
	"time"

	"github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/els0r/goProbe/pkg/goprobe/types"
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
	Stats types.PacketStats `json:"stats,omitempty"`
	Iface string            `json:"iface"`
}

// Manager manages a set of Capture instances.
// Each interface can be associated with up to one Capture.
type Manager struct {
	sync.Mutex
	captures map[string]*ManagedCapture
	ctx      context.Context
}

type ManagedCapture struct {
	capture *Capture
	cancel  context.CancelFunc
}

// NewManager creates a new Manager
func NewManager(ctx context.Context) *Manager {
	return &Manager{
		captures: make(map[string]*ManagedCapture),
		ctx:      ctx,
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
			// it's important that the parent context is background, since cancellation
			// of a parent context shouldn't propagate through and stop the capture, the
			// capture manager solely decides when it should be stopped
			capCtx, cancel := context.WithCancel(context.Background())

			capture := NewCapture(capCtx, iface, config)

			cm.setCapture(iface, &ManagedCapture{capture: capture, cancel: cancel})

			logger := logging.FromContext(capture.ctx)
			logger.Info(fmt.Sprintf("added interface to capture list"))

			capture.Run()
		}
	}
	rg.Wait()
}

// EnableAll attempts to enable all exisiting managed Capture instances.
func (cm *Manager) EnableAll() {
	var rg RunGroup

	cm.Lock()
	for _, mc := range cm.captures {
		mc := mc
		rg.Run(func() {
			mc.capture.Enable()
		})
	}
	cm.Unlock()

	rg.Wait()
	return
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
//
// If an instance has encountered an error or an instance's configuration
// differs from the one specified in ifaces, it will be re-enabled.
//
// Finally, if the Manager manages an instance for an iface that does
// not occur in ifaces, the following actions are performed on the instance:
//
// (1) the instance will be rotated,
// (2) the resulting flow data will be sent over returnChan,
// (tagged with the interface name and stats),
// (3) the instance will be closed,
// and (4) the instance will be completely removed from the Manager.
//
// Returns once all the above actions have been completed.
func (cm *Manager) Update(ifaces config.Ifaces, returnChan chan TaggedAggFlowMap) {
	logger := logging.FromContext(cm.ctx)

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
		logger = logging.FromContext(mc.capture.ctx)

		mc.cancel()
		cm.delCapture(iface)

		logger.Info(fmt.Sprintf("deleted interface from capture list"))
	}
	rg.Wait()

	elapsed := time.Since(t0).Round(time.Millisecond)

	logger.With("elapsed", elapsed.String()).Debug("updated interface list")

}

// Status returns the statuses of all interfaces provided in the arguments
func (cm *Manager) Status(ifaces ...string) map[string]types.InterfaceStatus {
	statusmapMutex := sync.Mutex{}
	statusmap := make(map[string]types.InterfaceStatus)

	var rg RunGroup
	cmCopy := cm.capturesCopy()

	if len(ifaces) > 0 {
		for _, iface := range ifaces {
			iface := iface
			mc, exists := cmCopy[iface]
			if exists {
				rg.Run(func() {
					status := mc.capture.Status()
					statusmapMutex.Lock()
					statusmap[iface] = status
					statusmapMutex.Unlock()
				})
			}
		}
	} else {
		for iface, mc := range cmCopy {
			iface, mc := iface, mc
			rg.Run(func() {
				status := mc.capture.Status()
				statusmapMutex.Lock()
				statusmap[iface] = status
				statusmapMutex.Unlock()
			})
		}
	}
	rg.Wait()

	return statusmap
}

// ActiveFlows returns a copy of the current in-memory flow map. If iface is "all", flows for every interface are returned
func (cm *Manager) ActiveFlows(ifaces ...string) map[string]types.FlowInfos {
	ifaceFlows := make(map[string]*types.FlowLog)
	ifaceFlowsMutex := sync.Mutex{}

	cmCopy := cm.capturesCopy()
	var rg RunGroup
	if len(ifaces) > 0 {
		for _, iface := range ifaces {
			mc, exists := cmCopy[iface]
			if exists {
				rg.Run(func() {
					f := mc.capture.Flows()
					ifaceFlowsMutex.Lock()
					ifaceFlows[iface] = f
					ifaceFlowsMutex.Unlock()
				})
			}
		}
	} else {
		for iface, mc := range cmCopy {
			iface, mc := iface, mc
			rg.Run(func() {
				f := mc.capture.Flows()
				ifaceFlowsMutex.Lock()
				ifaceFlows[iface] = f
				ifaceFlowsMutex.Unlock()
			})
		}
	}
	rg.Wait()

	// convert data
	var ifaceFlowInfos = make(map[string]types.FlowInfos, len(ifaceFlows))
	for iface, flowLog := range ifaceFlows {
		var flowInfos []types.FlowInfo
		for _, flow := range flowLog.Flows() {
			flowInfos = append(flowInfos, types.FlowInfo{
				Idle:                    flow.HasBeenIdle(),
				DirectionConfidenceHigh: flow.DirectionConfidenceHigh(),
				Flow:                    flow.ToExtendedRow(),
			})
		}

		// sort by packets, descending
		sort.SliceStable(flowInfos, func(i, j int) bool {
			return flowInfos[i].Flow.Counters.SumBytes() > flowInfos[j].Flow.Counters.SumBytes()
		})

		ifaceFlowInfos[iface] = flowInfos
	}

	return ifaceFlowInfos
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
	logger := logging.FromContext(cm.ctx)

	t0 := time.Now()

	var rg RunGroup

	logger.Debug("rotating all captures")

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

	logger.With("elapsed", elapsed.String()).Debug("completed rotation of all captures")
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

	logger.With("elapsed", elapsed.String()).Debug("closed all captures")
}
