package capture

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/els0r/goProbe/pkg/capture/capturetypes"
	"github.com/els0r/goProbe/pkg/logging"
	"github.com/els0r/goProbe/pkg/types/hashmap"
	"github.com/fako1024/slimcap/capture"
	"github.com/fako1024/slimcap/capture/afpacket/afring"
	"github.com/fako1024/slimcap/link"
)

const (

	// MaxIfaces is the maximum number of interfaces we can monitor
	MaxIfaces = 1024
)

var (

	// ErrLocalBufferOverflow signifies that the local packet buffer is full
	ErrLocalBufferOverflow = errors.New("local packet buffer overflow")

	defaultSourceInitFn = func(c *Capture) (capture.SourceZeroCopy, error) {
		return afring.NewSource(c.iface,
			afring.CaptureLength(link.CaptureLengthMinimalIPv6Transport),
			afring.BufferSize(c.config.RingBuffer.BlockSize, c.config.RingBuffer.NumBlocks),
			afring.Promiscuous(c.config.Promisc),
		)
	}
)

// sourceInitFn denotes the function used to initialize a capture source,
// providing the ability to override the default behavior, e.g. in mock tests
type sourceInitFn func(*Capture) (capture.SourceZeroCopy, error)

// Captures denotes a named set of Capture instances, wrapping a map and the
// required synchronization of all its actions
type captures struct {
	Map map[string]*Capture
	sync.RWMutex
}

// newCaptures instantiates a new, empty set of Captures
func newCaptures() *captures {
	return &captures{
		Map:     make(map[string]*Capture),
		RWMutex: sync.RWMutex{},
	}
}

// Ifaces return the list of names of all interfaces in the set
func (c *captures) Ifaces(ifaces ...string) []string {
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
func (c *captures) Get(iface string) (capture *Capture, exists bool) {
	c.RLock()
	capture, exists = c.Map[iface]
	c.RUnlock()
	return
}

// Set safely adds / overwrites a Capture by name
func (c *captures) Set(iface string, capture *Capture) {
	c.Lock()
	c.Map[iface] = capture
	c.Unlock()
}

// Delete safely removes a Capture from the set by name
func (c *captures) Delete(iface string) {
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
	flowLog *FlowLog

	// Generic handle / source for packet capture
	captureHandle capture.SourceZeroCopy
	sourceInitFn  sourceInitFn

	// Error tracking (type / errno specific)
	// parsingErrors ParsingErrTracker

	// WaitGroup tracking active processing
	wgProc sync.WaitGroup

	// startedAt tracks when the capture was started
	startedAt time.Time
}

// newCapture creates a new Capture associated with the given iface.
func newCapture(iface string, config config.CaptureConfig) *Capture {
	return &Capture{
		iface:        iface,
		config:       config,
		capLock:      newCaptureLock(),
		flowLog:      NewFlowLog(),
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

func (c *Capture) run() (err error) {

	// Set up the packet source and capturing
	c.captureHandle, err = c.sourceInitFn(c)
	if err != nil {
		return fmt.Errorf("failed to initialize capture: %w", err)
	}

	// make sure to store when the capture started
	c.startedAt = time.Now()

	return
}

func (c *Capture) close() error {
	if err := c.captureHandle.Close(); err != nil {
		return err
	}

	// Wait until processing has concluded
	c.wgProc.Wait()

	// Setting the handle to nil isn't stricly necessary, but it's an additional
	// guard against races (because it allows the race detector to pick up more
	// easily on potential concurrent accesses) and might trigger a crash on any
	// unwanted access
	c.captureHandle = nil
	return nil
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
func (c *Capture) process() <-chan error {

	captureErrors := make(chan error, 64)

	c.wgProc.Add(1)
	go func() {

		defer func() {
			close(captureErrors)
			c.wgProc.Done()
		}()

		// Main packet capture loop which an interface should be in most of the time
		localBuf := NewLocalBuffer(c.captureHandle)
		for {

			// Since lock confirmation is only done from a single goroutine (this one)
			// tracking if the capture source was unblocked is safe and can act as flag when to
			// read from the lock request channel (which in turn is atomic).
			// Similarly, once this goroutine observes that the channel length is 1 it is guaranteed
			// that there is a request on the channel that can be read on the next line.
			// This logic may be slightly more contrived than a select{} statement but it increases
			// packet throughput by several percent points
			if len(c.capLock.request) > 0 {
				buf := <-c.capLock.request      // Consume the lock request
				c.capLock.confirm <- struct{}{} // Confirm that process() is not processing

				// Claim / assign the shared data from the memory pool for / to this buffer
				localBuf.Assign(buf)

				// Continue fetching packets and add them to the local buffer
				for {
					if len(c.capLock.done) > 0 {
						<-c.capLock.done // Consume the unlock request to continue normal processing
						break
					}

					// Fetch the next packet form the wire
					ipLayer, pktType, pktSize, err := c.captureHandle.NextIPPacketZeroCopy()
					if err != nil {

						// If we receive an unblock event while capturing to buffer, continue
						if errors.Is(err, capture.ErrCaptureUnblocked) { // capture unblocked (during lock)
							continue
						}
						if errors.Is(err, capture.ErrCaptureStopped) { // capture stopped gracefully
							localBuf.Release()
							return
						}

						localBuf.Release()
						captureErrors <- fmt.Errorf("capture error while buffering: %w", err)
						return
					}

					// Try to append to local buffer. In case the buffer is full, stop buffering and
					// wait for the unlock request
					if !localBuf.Add(ipLayer, pktType, pktSize) {
						captureErrors <- ErrLocalBufferOverflow
						<-c.capLock.done // Consume the unlock request to continue normal processing
						break
					}
				}

				// Drain buffer if not empty
				if localBuf.N() > 0 {
					for i := 0; i < localBuf.N(); i++ {
						c.addToFlowLog(localBuf.Get(i))
					}
				}
				localBuf.Release()

				// Advance to the next loop iteration in case there is a pending lock
				continue
			}

			// Fetch the next packet or PPOLL event from the source
			if err := c.capturePacket(); err != nil {
				if errors.Is(err, capture.ErrCaptureUnblocked) { // capture unblocked

					// Advance to the next loop iteration (during which the pending lock will be
					// consumed / acted on)
					continue
				}
				if errors.Is(err, capture.ErrCaptureStopped) { // capture stopped gracefully
					return
				}

				captureErrors <- err
				return
			}
		}
	}()

	return captureErrors
}

func (c *Capture) capturePacket() error {

	// Fetch the next packet form the wire
	ipLayer, pktType, pktSize, err := c.captureHandle.NextIPPacketZeroCopy()
	if err != nil {

		// NextPacket should return a ErrCaptureStopped in case the handle is closed or
		// ErrCaptureUnblock in case the PPOLL was unblocked
		return fmt.Errorf("capture error: %w", err)
	}

	c.addToFlowLog(ipLayer, pktType, pktSize)
	return nil
}

func (c *Capture) addToFlowLog(ipLayer capture.IPLayer, pktType capture.PacketType, pktSize uint32) {

	// Parse / add the received data to the map of flows
	errno := c.flowLog.Add(ipLayer, pktType, pktSize)
	c.stats.Processed++
	if errno == capturetypes.ErrnoOK {
		return
	}

	if errno.ParsingFailed() {
		c.stats.ParsingErrors[errno]++
	}
}

func (c *Capture) status() (*capturetypes.CaptureStats, error) {

	stats, err := c.captureHandle.Stats()
	if err != nil {
		return nil, err
	}

	c.stats.ReceivedTotal += stats.PacketsReceived
	c.stats.ProcessedTotal += c.stats.Processed
	c.stats.DroppedTotal += stats.PacketsDropped

	// add exposed metrics
	// we do this every 5 minutes only in order not to interfere with the
	// main packet processing loop. If this counter moves slowly (as in gets
	// gets an update only every 5 minutes) it's not an issue to understand
	// processed data volumes across longer time frames
	packetsProcessed.Add(float64(c.stats.Processed))
	packetsDropped.Add(float64(stats.PacketsDropped))
	captureErrors.Add(float64(c.stats.ParsingErrors.Sum()))

	res := capturetypes.CaptureStats{
		StartedAt:      c.startedAt,
		Received:       stats.PacketsReceived,
		ReceivedTotal:  c.stats.ReceivedTotal,
		Processed:      c.stats.Processed,
		ProcessedTotal: c.stats.ProcessedTotal,
		Dropped:        stats.PacketsDropped,
		DroppedTotal:   c.stats.DroppedTotal,
		ParsingErrors:  c.stats.ParsingErrors,
	}

	c.stats.Processed = 0
	c.stats.ParsingErrors.Reset()

	return &res, nil
}

func (c *Capture) fetchStatusInBackground(ctx context.Context) (res chan *capturetypes.CaptureStats) {
	res = make(chan *capturetypes.CaptureStats)

	// Extract capture stats in a separate goroutine to minimize time-to-unblock
	// This should be finished by the time the rotation has taken place (at which
	// time the stats can be pulled from the returned channel)
	go func() {
		stats, err := c.status()
		if err != nil {
			logging.FromContext(ctx).Errorf("failed to get capture stats: %v", err)
		}

		res <- stats
		close(res)
	}()

	return
}

func (c *Capture) lock() {

	// Fetch data from the pool for the local buffer. Tis will wait until it is actually
	// available, allowing us to use a single buffer for all interfaces
	buf := memPool.Get(0)

	// Notify the capture that a locked interaction is about to begin, then
	// unblock the capture potentially being in a blocking PPOLL syscall
	// Channel has a depth of one and hence this push is non-blocking. Since
	// we wait for confirmation there is no possibility of repeated attempts
	// or race conditions
	c.capLock.request <- buf
	if err := c.captureHandle.Unblock(); err != nil {
		panic(fmt.Sprintf("unexpectedly failed to unblock capture handle, deadlock inevitable: %s", err))
	}

	// Wait for confirmation of reception from the processing routine
	<-c.capLock.confirm
}

func (c *Capture) unlock() {

	// Signal that the rotation is complete, releasing the processing routine
	// Since the done channel has a depth of one an Unblock() event needs to be
	// sent to ensure that a capture currently waiting for packets in the buffering
	// state continues to the next iteration in order to observe the unlock request
	c.capLock.done <- struct{}{}
	if err := c.captureHandle.Unblock(); err != nil {
		panic(fmt.Sprintf("unexpectedly failed to unblock capture handle, deadlock inevitable: %s", err))
	}
}

type captureLock struct {
	request chan []byte
	confirm chan struct{}
	done    chan struct{}
}

func newCaptureLock() *captureLock {
	return &captureLock{
		request: make(chan []byte, 1),
		confirm: make(chan struct{}),
		done:    make(chan struct{}, 1),
	}
}
