package capture

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/els0r/goProbe/pkg/capture/capturetypes"
	"github.com/els0r/goProbe/pkg/logging"
	"github.com/els0r/goProbe/pkg/types/hashmap"
	"github.com/fako1024/slimcap/capture"
	"github.com/fako1024/slimcap/capture/afpacket/afring"
	"github.com/fako1024/slimcap/link"
	"golang.org/x/exp/slog"
)

const (

	// MaxIfaces is the maximum number of interfaces we can monitor
	MaxIfaces = 1024

	// ErrorThreshold is the maximum amount of consecutive errors that can occur on an interface before capturing is halted.
	ErrorThreshold = 10000
)

var defaultSourceInitFn = func(c *Capture) (capture.Source, error) {
	return afring.NewSource(c.iface,
		afring.CaptureLength(link.CaptureLengthMinimalIPv6Transport),
		afring.BufferSize(c.config.RingBufferBlockSize, c.config.RingBufferNumBlocks),
		afring.Promiscuous(c.config.Promisc),
	)
}

// sourceInitFn denotes the function used to initialize a capture source,
// providing the ability to override the default behavior, e.g. in mock tests
type sourceInitFn func(*Capture) (capture.Source, error)

// Captures denotes a named set of Capture instances, wrapping a map and the
// required synchronization of all its actions
type Captures struct {
	Map map[string]*Capture
	sync.RWMutex
}

// NewCaptures instantiates a new, empty set of Captures
func NewCaptures() *Captures {
	return &Captures{
		Map:     make(map[string]*Capture),
		RWMutex: sync.RWMutex{},
	}
}

// Ifaces return the list of names of all interfaces in the set
func (c *Captures) Ifaces(ifaces ...string) []string {
	if len(ifaces) == 0 {
		c.RLock()
		ifaces = make([]string, 0, len(c.Map))
		for iface := range c.Map {
			ifaces = append(ifaces, iface)
		}
		c.RUnlock()
	}

	return ifaces
}

// Get safely returns a Capture by name (and an indicator if it exists)
func (c *Captures) Get(iface string) (capture *Capture, exists bool) {
	c.RLock()
	capture, exists = c.Map[iface]
	c.RUnlock()
	return
}

// Set safely adds / overwrites a Capture by name
func (c *Captures) Set(iface string, capture *Capture) {
	c.Lock()
	c.Map[iface] = capture
	c.Unlock()
}

// Delete safely removes a Capture from the set by name
func (c *Captures) Delete(iface string) {
	c.Lock()
	delete(c.Map, iface)
	c.Unlock()
}

// Capture captures and logs flow data for all traffic on a
// given network interface. For each Capture, a goroutine is
// spawned at creation time. To avoid leaking this goroutine,
// be sure to call Close() when you're done with a Capture.
//
// Each capture is associated with a network interface when created. This interface
// can never be changed.
//
// All public methods of Capture are threadsafe.
type Capture struct {
	iface string

	config config.CaptureConfig

	// stats from the last rotation or reset (needed for Status)
	stats capturetypes.CaptureStats

	// Rotation state synchronization
	capLock *captureLock

	// Logged flows since creation of the capture (note that some
	// flows are retained even after Rotate has been called)
	flowLog *capturetypes.FlowLog

	// Generic handle / source for packet capture
	captureHandle capture.Source
	sourceInitFn  sourceInitFn

	// error map for logging errors more properly
	errMap   capturetypes.ErrorMap
	errCount int
}

// newCapture creates a new Capture associated with the given iface.
func newCapture(iface string, config config.CaptureConfig) *Capture {
	return &Capture{
		iface:        iface,
		config:       config,
		capLock:      newCaptureLock(),
		flowLog:      capturetypes.NewFlowLog(),
		errMap:       make(map[string]int),
		sourceInitFn: defaultSourceInitFn,
	}
}

// SetSourceInitFn sets a custom function used to initialize a new capture
func (c *Capture) SetSourceInitFn(fn sourceInitFn) *Capture {
	c.sourceInitFn = fn
	return c
}

// Iface returns the name of the interface
func (c *Capture) Iface() string {
	return c.iface
}

func (c *Capture) run(ctx context.Context) (err error) {

	ctx = logging.WithFields(ctx, slog.String("iface", c.iface))
	logger := logging.FromContext(ctx)
	logger.Info("initializing capture / running packet processing")

	// Set up the packet source and capturing
	c.captureHandle, err = c.sourceInitFn(c)
	if err != nil {
		return fmt.Errorf("failed to initialize capture: %w", err)
	}

	// Start up processing and error handling / logging in the
	// background
	go logErrors(ctx,
		c.process(ctx))

	return
}

func (c *Capture) close() error {
	return c.captureHandle.Close()
}

func (c *Capture) rotate(ctx context.Context) (agg *hashmap.AggFlowMap) {

	logger := logging.FromContext(ctx)

	if c.flowLog.Len() == 0 {
		logger.Debug("there are currently no flow records available")
		return
	}
	agg = c.flowLog.Rotate()

	return
}

// process is the heart of the Capture. It listens for network traffic on the
// network interface and logs the corresponding flows.
//
// process keeps running until Close is called on its capture handle or it encounters
// a serious capture error
func (c *Capture) process(ctx context.Context) <-chan error {

	captureErrors := make(chan error, 64)

	go func() {
		logger := logging.FromContext(ctx)
		c.errCount = 0

		// Reusable packet buffer for in-place population
		pkt := c.captureHandle.NewPacket()

		// Main packet capture loop which an interface should be in most of the time
		for {

			// map access needs to be synchronized. This will incur a performance hit, but less so
			// than locking the data structure itself (mutex causes about 40% drop in throughput)
			select {

			// Lock state synchronization
			case <-c.capLock.request:
				c.capLock.confirm <- struct{}{} // Confirm that process() is not processing
				<-c.capLock.done

			// Normal processing
			default:
				err := c.capturePacket(pkt)
				if err != nil {
					if errors.Is(err, capture.ErrCaptureUnblock) { // capture unblocked (e.g. during rotation)
						continue
					}
					if errors.Is(err, capture.ErrCaptureStopped) { // capture stopped gracefully
						if err := c.captureHandle.Free(); err != nil {
							logger.Errorf("failed to free capture resources: %s", err)
						}
						c.captureHandle = nil

						return
					}

					captureErrors <- err
					return
				}
			}
		}
	}()

	return captureErrors
}

func (c *Capture) capturePacket(pkt capture.Packet) (err error) {

	// Fetch the next packet form the wire
	_, err = c.captureHandle.NextPacket(pkt)
	if err != nil {

		// NextPacket should return a ErrCaptureStopped in case the handle is closed or
		// ErrCaptureUnblock in case the PPOLL was unblocked
		return fmt.Errorf("capture error: %w", err)
	}

	// Populate the capturetypes.GPPacket
	// Instead of reusing an instance of capture.Packet over and over again a new
	// one is allocated for each run on capturePacket(). Since it does not escape it is
	// allocated on the stack and hence does not cause any GC overhead (and allocating is
	// actually faster than resetting its fields, plus in that case it escapes to the heap)
	var gppacket capturetypes.GPPacket
	if err = Populate(&gppacket, pkt); err == nil {
		c.flowLog.Add(&gppacket)
		c.stats.Processed++
		c.errCount = 0
		return nil
	}

	c.stats.Processed++
	c.errCount++
	c.errMap[err.Error()]++

	// Shut down the interface thread if too many consecutive decoding failures
	// have been encountered
	if c.errCount > ErrorThreshold {
		return fmt.Errorf("the last %d packets could not be decoded: [%s]",
			ErrorThreshold,
			c.errMap.String(),
		)
	}

	return nil
}

func (c *Capture) status() (*capturetypes.CaptureStats, error) {

	stats, err := c.captureHandle.Stats()
	if err != nil {
		return nil, err
	}

	c.stats.ReceivedTotal += stats.PacketsReceived
	c.stats.ProcessedTotal += c.stats.Processed

	res := capturetypes.CaptureStats{
		Received:       stats.PacketsReceived,
		ReceivedTotal:  c.stats.ReceivedTotal,
		Dropped:        stats.PacketsDropped,
		Processed:      c.stats.Processed,
		ProcessedTotal: c.stats.ProcessedTotal,
	}
	c.stats.Processed = 0

	return &res, nil
}

func (c *Capture) lock() {

	// Notify the capture that a locked interaction is about to begin, then
	// unblock the capture potentially being in a blocking PPOLL syscall
	// Channel has a depth of one and hence this push is non-blocking. Since
	// we wait for confirmation there is no possibility of repeated attempts
	// or race conditions
	c.capLock.request <- struct{}{}
	if err := c.captureHandle.Unblock(); err != nil {
		panic(fmt.Sprintf("unexpectedly failed to unblock capture handle, deadlock inevitable: %s", err))
	}

	// Wait for confirmation of reception from the processing routine, then
	// commit the rotation
	<-c.capLock.confirm
}

func (c *Capture) unlock() {

	// Signal that the rotation is complete, releasing the processing routine
	c.capLock.done <- struct{}{}
}

type captureLock struct {
	request chan struct{}
	confirm chan struct{}
	done    chan struct{}
}

func newCaptureLock() *captureLock {
	return &captureLock{
		request: make(chan struct{}, 1),
		confirm: make(chan struct{}),
		done:    make(chan struct{}),
	}
}

func logErrors(ctx context.Context, errsChan <-chan error) {
	logger := logging.FromContext(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errsChan:
			logger.Error(err)
			return
		}
	}
}
