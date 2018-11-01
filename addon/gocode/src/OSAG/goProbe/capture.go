/////////////////////////////////////////////////////////////////////////////////
//
// capture.go
//
// Written by Lorenz Breidenbach lob@open.ch, December 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package goProbe

import (
	"fmt"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"

	"OSAG/goDB"
)

const (
	CAPTURE_SNAPLEN         = 86
	CAPTURE_ERROR_THRESHOLD = 10000
	// Our experiments show that you don't want to set this value lower
	// than roughly 100 ms. Otherwise we flood the kernel with syscalls
	// and our performance drops.
	CAPTURE_TIMEOUT time.Duration = 500 * time.Millisecond

	MIN_PCAP_BUF_SIZE = 1024               // require at least one KiB
	MAX_PCAP_BUF_SIZE = 1024 * 1024 * 1024 // 1 GiB should be enough for anyone ;)
)

//////////////////////// Ancillary types ////////////////////////

type CaptureConfig struct {
	BufSize   int    `json:"buf_size"` // in bytes
	BPFFilter string `json:"bpf_filter"`
	Promisc   bool   `json:"promisc"`
}

// Validate (partially) checks that the given CaptureConfig contains no bogus settings.
//
// Note that the BPFFilter field isn't checked.
func (cc CaptureConfig) Validate() error {
	if !(MIN_PCAP_BUF_SIZE <= cc.BufSize && cc.BufSize <= MAX_PCAP_BUF_SIZE) {
		return fmt.Errorf("Invalid configuration entry BufSize. Value must be in range [%d, %d].", MIN_PCAP_BUF_SIZE, MAX_PCAP_BUF_SIZE)
	}
	return nil
}

type CaptureState byte

const (
	CAPTURE_STATE_UNINITIALIZED CaptureState = iota + 1
	CAPTURE_STATE_INITIALIZED
	CAPTURE_STATE_ACTIVE
	CAPTURE_STATE_ERROR
)

func (cs CaptureState) String() string {
	switch cs {
	case CAPTURE_STATE_UNINITIALIZED:
		return "CAPTURE_STATE_UNINITIALIZED"
	case CAPTURE_STATE_INITIALIZED:
		return "CAPTURE_STATE_INITIALIZED"
	case CAPTURE_STATE_ACTIVE:
		return "CAPTURE_STATE_ACTIVE"
	case CAPTURE_STATE_ERROR:
		return "CAPTURE_STATE_ERROR"
	default:
		return "Unknown"
	}
}

type CaptureStats struct {
	Pcap          *pcap.Stats
	PacketsLogged int
}

type CaptureStatus struct {
	State CaptureState
	Stats CaptureStats
}

type errorMap map[string]int

func (e errorMap) String() string {
	var str string
	for err, count := range e {
		str += fmt.Sprintf(" %s(%d);", err, count)
	}
	return str
}

//////////////////////// capture commands ////////////////////////

// captureCommand is an interface implemented by (you guessed it...)
// all capture commands. A capture command is sent to the process() of
// a Capture over the Capture's cmdChan. The captureCommand's execute()
// method is then executed by process() (and in process()'s goroutine).
// As a result we don't have to worry about synchronization of the
// Capture's pcap handle inside the execute() methods.
type captureCommand interface {
	// executes the command on the provided capture instance.
	// This will always be called from the process() goroutine.
	execute(c *Capture)
}

type captureCommandStatus struct {
	returnChan chan<- CaptureStatus
}

type captureCommandErrors struct {
	returnChan chan<- errorMap
}

func (cmd captureCommandStatus) execute(c *Capture) {
	var result CaptureStatus

	result.State = c.state

	pcapStats := c.tryGetPcapStats()
	result.Stats = CaptureStats{
		Pcap:          subPcapStats(pcapStats, c.lastRotationStats.Pcap),
		PacketsLogged: c.packetsLogged - c.lastRotationStats.PacketsLogged,
	}

	cmd.returnChan <- result
}

func (cmd captureCommandErrors) execute(c *Capture) {
	cmd.returnChan <- c.errMap
}

type captureCommandUpdate struct {
	config     CaptureConfig
	returnChan chan<- struct{}
}

func (cmd captureCommandUpdate) execute(c *Capture) {
	if c.state == CAPTURE_STATE_ACTIVE {
		if c.needReinitialization(cmd.config) {
			c.deactivate()
		} else {
			cmd.returnChan <- struct{}{}
			return
		}
	}

	// Can no longer be in CAPTURE_STATE_ACTIVE at this point
	// Now try to make Capture initialized with new config.
	switch c.state {
	case CAPTURE_STATE_UNINITIALIZED:
		c.config = cmd.config
		c.initialize()
	case CAPTURE_STATE_INITIALIZED:
		if c.needReinitialization(cmd.config) {
			c.uninitialize()
			c.config = cmd.config
			c.initialize()
		}
	case CAPTURE_STATE_ERROR:
		c.recoverError()
		c.config = cmd.config
		c.initialize()
	}

	SysLog.Debug(fmt.Sprintf("Interface '%s': (re)initialized for configuration update", c.iface))

	// If initialization in last step succeeded, activate
	if c.state == CAPTURE_STATE_INITIALIZED {
		c.activate()
	}

	cmd.returnChan <- struct{}{}
}

// helper struct to bundle up the multiple return values
// of Rotate
type rotateResult struct {
	agg   goDB.AggFlowMap
	stats CaptureStats
}

type captureCommandRotate struct {
	returnChan chan<- rotateResult
}

func (cmd captureCommandRotate) execute(c *Capture) {
	var result rotateResult

	result.agg = c.flowLog.Rotate()

	pcapStats := c.tryGetPcapStats()

	result.stats = CaptureStats{
		Pcap:          subPcapStats(pcapStats, c.lastRotationStats.Pcap),
		PacketsLogged: c.packetsLogged - c.lastRotationStats.PacketsLogged,
	}

	c.lastRotationStats = CaptureStats{
		Pcap:          pcapStats,
		PacketsLogged: c.packetsLogged,
	}

	cmd.returnChan <- result
}

type captureCommandEnable struct {
	returnChan chan<- struct{}
}

func (cmd captureCommandEnable) execute(c *Capture) {
	update := captureCommandUpdate{
		c.config,
		cmd.returnChan,
	}
	update.execute(c)
}

type captureCommandDisable struct {
	returnChan chan<- struct{}
}

func (cmd captureCommandDisable) execute(c *Capture) {
	switch c.state {
	case CAPTURE_STATE_UNINITIALIZED:
	case CAPTURE_STATE_INITIALIZED:
		c.uninitialize()
	case CAPTURE_STATE_ACTIVE:
		c.deactivate()
		c.uninitialize()
	case CAPTURE_STATE_ERROR:
		c.recoverError()
	}

	cmd.returnChan <- struct{}{}
}

// BUG(pcap): There is a pcap bug? that causes mysterious panics
// when we try to call Activate on more than one pcap.InactiveHandle
// at the same time.
// We have also observed (much rarer) panics triggered by calls to
// SetBPFFilter on activated pcap handles.
// Hence we use PcapMutex to make sure that
// there can only be on call to Activate and SetBPFFilter at any given
// moment.

// This mutex linearizes all pcap.InactiveHandle.Activate and
// pcap.Handle.SetBPFFilter calls. Don't touch it unless you know what you're
// doing.
var PcapMutex sync.Mutex

//////////////////////// Capture definition ////////////////////////

// A Capture captures and logs flow data for all traffic on a
// given network interface. For each Capture, a goroutine is
// spawned at creation time. To avoid leaking this goroutine,
// be sure to call Close() when you're done with a Capture.
//
// Each Capture is a finite state machine.
// Here is a diagram of the possible state transitions:
//
//           +---------------+
//           |               |
//           |               |
//           |               +---------------------+
//           |               |                     |
//           | UNINITIALIZED <-------------------+ |
//           |               |  recoverError()   | |
//           +----^-+--------+                   | |initialize()
//                | |                            | |fails
//                | |initialize() is             | |
//                | |successful                  | |
//                | |                            | |
//  uninitialize()| |                            | |
//                | |                            | |
//            +---+-v-------+                    | |
//            |             |                +---+-v---+
//            |             |                |         |
//            |             |                |         |
//            |             |                |  ERROR  |
//            | INITIALIZED |                |         |
//            |             |                +----^----+
//            +---^-+-------+                     |
//                | |                             |
//                | |activate()                   |
//                | |                             |
//    deactivate()| |                             |
//                | |                             |
//              +-+-v----+                        |
//              |        |                        |
//              |        +------------------------+
//              |        |  capturePacket()
//              |        |  (called by process())
//              | ACTIVE |  fails
//              |        |
//              +--------+
//
// Enable() and Update() try to put the capture into the ACTIVE state, Disable() puts the capture
// into the UNINITIALIZED state.
//
// Each capture is associated with a network interface when created. This interface
// can never be changed.
//
// All public methods of Capture are threadsafe.
type Capture struct {
	iface string
	// synchronizes all access to the Capture's public methods
	mutex sync.Mutex
	// has Close been called on the Capture?
	closed bool

	state CaptureState

	config CaptureConfig

	// channel over which commands are passed to process()
	// close(cmdChan) is used to tell process() to stop
	cmdChan chan captureCommand

	// stats from the last rotation or reset (needed for Status)
	lastRotationStats CaptureStats

	// Counts the total number of logged packets (since the creation of the
	// Capture)
	packetsLogged int

	// Logged flows since creation of the capture (note that some
	// flows are retained even after Rotate has been called)
	flowLog *FlowLog

	pcapHandle   *pcap.Handle
	packetSource *gopacket.PacketSource

	// error map for logging errors more properly
	errMap errorMap
}

// NewCapture creates a new Capture associated with the given iface.
func NewCapture(iface string, config CaptureConfig) *Capture {
	c := &Capture{
		iface,
		sync.Mutex{},
		false, // closed
		CAPTURE_STATE_UNINITIALIZED,
		config,
		make(chan captureCommand, 1),
		CaptureStats{
			Pcap:          &pcap.Stats{},
			PacketsLogged: 0,
		},
		0, // packetsLogged
		NewFlowLog(),
		nil, // pcapHandle
		nil, // packetSource
		make(map[string]int),
	}
	go c.process()
	return c
}

// setState provides write access to the state field of
// a Capture. It also logs the state change.
func (c *Capture) setState(s CaptureState) {
	c.state = s
	SysLog.Debug(fmt.Sprintf("Interface '%s': entered capture state %s", c.iface, s))
}

// process is the heart of the Capture. It listens for network traffic on the
// network interface and logs the corresponding flows.
//
// As long as the Capture is in CAPTURE_STATE_ACTIVE process() is capturing
// packets from the network. In any other state, process() only awaits
// further commands.
//
// process keeps running its own goroutine until Close is called on its Capture.
func (c *Capture) process() {
	errcount := 0
	gppacket := GPPacket{}

	capturePacket := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				trace := string(debug.Stack())
				fmt.Fprintf(os.Stderr, "Interface '%s': panic returned %v. Stacktrace:\n%s\n", c.iface, r, trace)
				err = fmt.Errorf("Panic during capture")
				return
			}
		}()

		packet, err := c.packetSource.NextPacket()
		if err != nil {
			if err == pcap.NextErrorTimeoutExpired { // CAPTURE_TIMEOUT expired
				return nil
			} else {
				return fmt.Errorf("Capture error: %s", err)
			}
		}

		if err := gppacket.Populate(packet); err == nil {
			c.flowLog.Add(&gppacket)
			errcount = 0
			c.packetsLogged++
		} else {
			errcount++

			// collect the error. The errors value is the key here. Otherwise, the address
			// of the error would be taken, which results in a non-minimal set of errors
			if _, exists := c.errMap[err.Error()]; !exists {
				// log the packet to the pcap error logs
				if logerr := PacketLog.Log(c.iface, packet, CAPTURE_SNAPLEN); logerr != nil {
					SysLog.Info("failed to log faulty packet: " + logerr.Error())
				}
			}

			c.errMap[err.Error()]++

			// shut down the interface thread if too many consecutive decoding failures
			// have been encountered
			if errcount > CAPTURE_ERROR_THRESHOLD {
				return fmt.Errorf("The last %d packets could not be decoded: [%s ]",
					CAPTURE_ERROR_THRESHOLD,
					c.errMap.String(),
				)
			}
		}

		return nil
	}

	for {
		if c.state == CAPTURE_STATE_ACTIVE {
			if err := capturePacket(); err != nil {
				c.setState(CAPTURE_STATE_ERROR)
				SysLog.Err(fmt.Sprintf("Interface '%s': %s", c.iface, err.Error()))
			}

			select {
			case cmd, ok := <-c.cmdChan:
				if ok {
					cmd.execute(c)
				} else {
					return
				}
			default:
				// keep going
			}
		} else {
			cmd, ok := <-c.cmdChan
			if ok {
				cmd.execute(c)
			} else {
				return
			}
		}
	}
}

//////////////////////// state transisition functions ////////////////////////

// initialize attempts to transition from CAPTURE_STATE_UNINITIALIZED
// into CAPTURE_STATE_INITIALIZED. If an error occurrs, it instead
// transitions into state CAPTURE_STATE_ERROR.
func (c *Capture) initialize() {
	initializationErr := func(msg string, args ...interface{}) {
		SysLog.Err(fmt.Sprintf(msg, args...))
		c.setState(CAPTURE_STATE_ERROR)
		return
	}

	if c.state != CAPTURE_STATE_UNINITIALIZED {
		panic("Need state CAPTURE_STATE_UNINITIALIZED")
	}

	var err error

	inactiveHandle, err := setupInactiveHandle(c.iface, c.config.BufSize, c.config.Promisc)
	if err != nil {
		initializationErr("Interface '%s': failed to create inactive handle: %s", c.iface, err)
		return
	}
	defer inactiveHandle.CleanUp()

	PcapMutex.Lock()
	c.pcapHandle, err = inactiveHandle.Activate()
	PcapMutex.Unlock()
	if err != nil {
		initializationErr("Interface '%s': failed to activate handle: %s", c.iface, err)
		return
	}

	// link type might be null if the
	// specified interface does not exist (anymore)
	if c.pcapHandle.LinkType() == layers.LinkTypeNull {
		initializationErr("Interface '%s': has link type null", c.iface)
		return
	}

	PcapMutex.Lock()
	err = c.pcapHandle.SetBPFFilter(c.config.BPFFilter)
	PcapMutex.Unlock()
	if err != nil {
		initializationErr("Interface '%s': failed to set bpf filter to %s: %s", c.iface, c.config.BPFFilter, err)
		return
	}

	c.packetSource = gopacket.NewPacketSource(c.pcapHandle, c.pcapHandle.LinkType())

	// set the decoding options to lazy decoding in order to ensure that the packet
	// layers are only decoded once they are needed. Additionally, this is imperative
	// when GRE-encapsulated packets are decoded because otherwise the layers cannot
	// be detected correctly.
	// In addition to lazy decoding, the zeroCopy feature is enabled to avoid allocation
	// of a full copy of each gopacket, just to copy over a few elements into a GPPacket
	// structure afterwards.
	c.packetSource.DecodeOptions = gopacket.DecodeOptions{Lazy: true, NoCopy: true}

	c.setState(CAPTURE_STATE_INITIALIZED)
}

// uninitialize moves from CAPTURE_STATE_INITIALIZED to CAPTURE_STATE_UNINITIALIZED.
func (c *Capture) uninitialize() {
	if c.state != CAPTURE_STATE_INITIALIZED {
		panic("Need state CAPTURE_STATE_INITIALIZED")
	}
	c.reset()
}

// activate transitions from CAPTURE_STATE_INITIALIZED
// into CAPTURE_STATE_ACTIVE.
func (c *Capture) activate() {
	if c.state != CAPTURE_STATE_INITIALIZED {
		panic("Need state CAPTURE_STATE_INITIALIZED")
	}
	c.setState(CAPTURE_STATE_ACTIVE)
	SysLog.Debug(fmt.Sprintf("Interface '%s': capture active. Link type: %s", c.iface, c.pcapHandle.LinkType()))
}

// deactivate transitions from CAPTURE_STATE_ACTIVE
// into CAPTURE_STATE_INITIALIZED.
func (c *Capture) deactivate() {
	if c.state != CAPTURE_STATE_ACTIVE {
		panic("Need state CAPTURE_STATE_ACTIVE")
	}
	c.setState(CAPTURE_STATE_INITIALIZED)
	SysLog.Debug(fmt.Sprintf("Interface '%s': deactivated", c.iface))
}

// recoverError transitions from CAPTURE_STATE_ERROR
// into CAPTURE_STATE_UNINITIALIZED
func (c *Capture) recoverError() {
	if c.state != CAPTURE_STATE_ERROR {
		panic("Need state CAPTURE_STATE_ERROR")
	}
	c.reset()
}

//////////////////////// utilities ////////////////////////

// reset unites logic used in both recoverError and uninitialize
// in a single method.
func (c *Capture) reset() {
	if c.pcapHandle != nil {
		c.pcapHandle.Close()
	}
	// We reset the Pcap part of the stats because we will create
	// a new pcap handle with new counts when the Capture is next
	// initialized. We don't reset the PacketsLogged field because
	// it corresponds to the number of packets in the (untouched)
	// flowLog.
	c.lastRotationStats.Pcap = &pcap.Stats{}
	c.pcapHandle = nil
	c.packetSource = nil
	c.setState(CAPTURE_STATE_UNINITIALIZED)

	// reset the error map. The GC will take care of the previous
	// one
	c.errMap = make(map[string]int)
}

// needReinitialization checks whether we need to reinitialize the capture
// to apply the given config.
func (c *Capture) needReinitialization(config CaptureConfig) bool {
	return c.config != config
}

func (c *Capture) tryGetPcapStats() *pcap.Stats {
	var (
		pcapStats *pcap.Stats
		err       error
	)
	if c.pcapHandle != nil {
		pcapStats, err = c.pcapHandle.Stats()
		if err != nil {
			SysLog.Err(fmt.Sprintf("Interface '%s': error while requesting pcap stats: %s", err.Error()))
		}
	}
	return pcapStats
}

// subPcapStats computes a - b (fieldwise) if both a and b
// are not nil. Otherwise, it returns nil.
func subPcapStats(a, b *pcap.Stats) *pcap.Stats {
	if a == nil || b == nil {
		return nil
	} else {
		return &pcap.Stats{
			PacketsReceived:  a.PacketsReceived - b.PacketsReceived,
			PacketsDropped:   a.PacketsDropped - b.PacketsDropped,
			PacketsIfDropped: a.PacketsIfDropped - b.PacketsIfDropped,
		}
	}
}

// setupInactiveHandle sets up a pcap InactiveHandle with the given settings.
func setupInactiveHandle(iface string, bufSize int, promisc bool) (*pcap.InactiveHandle, error) {
	// new inactive handle
	inactive, err := pcap.NewInactiveHandle(iface)
	if err != nil {
		inactive.CleanUp()
		return nil, err
	}

	// set up buffer size
	if err := inactive.SetBufferSize(bufSize); err != nil {
		inactive.CleanUp()
		return nil, err
	}

	// set snaplength
	if err := inactive.SetSnapLen(int(CAPTURE_SNAPLEN)); err != nil {
		inactive.CleanUp()
		return nil, err
	}

	// set promisc mode
	if err := inactive.SetPromisc(promisc); err != nil {
		inactive.CleanUp()
		return nil, err
	}

	// set timeout
	if err := inactive.SetTimeout(CAPTURE_TIMEOUT); err != nil {
		inactive.CleanUp()
		return nil, err
	}

	// return the inactive handle for activation
	return inactive, err
}

//////////////////////// public functions ////////////////////////

// Status returns the current CaptureState as well as the statistics
// collected since the last call to Rotate()
//
// Note: If the Capture was reinitialized since the last rotation,
// result.Stats.Pcap will be inaccurate.
//
// Note: result.Stats.Pcap may be null if there was an error fetching the
// stats of the underlying pcap handle.
func (c *Capture) Status() (result CaptureStatus) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.closed {
		panic("Capture is closed")
	}

	ch := make(chan CaptureStatus, 1)
	c.cmdChan <- captureCommandStatus{ch}
	return <-ch
}

// Error map status call
func (c *Capture) Errors() (result errorMap) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.closed {
		panic("Capture is closed")
	}

	ch := make(chan errorMap, 1)
	c.cmdChan <- captureCommandErrors{ch}
	return <-ch
}

// Update will attempt to put the Capture instance into
// CAPTURE_STATE_ACTIVE with the given config.
// If the Capture is already active with the given config
// Update will detect this and do no work.
func (c *Capture) Update(config CaptureConfig) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.closed {
		panic("Capture is closed")
	}

	ch := make(chan struct{}, 1)
	c.cmdChan <- captureCommandUpdate{config, ch}
	<-ch
}

// Enable will attempt to put the Capture instance into
// CAPTURE_STATE_ACTIVE.
// Enable will have no effect if the Capture is already
// in CAPTURE_STATE_ACTIVE.
func (c *Capture) Enable() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.closed {
		panic("Capture is closed")
	}

	ch := make(chan struct{}, 1)
	c.cmdChan <- captureCommandEnable{ch}
	<-ch
}

// Disable will bring the Capture instance into CAPTURE_STATE_UNINITIALIZED
// Disable will have no effect if the Capture is already
// in CAPTURE_STATE_UNINITIALIZED.
func (c *Capture) Disable() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.closed {
		panic("Capture is closed")
	}

	ch := make(chan struct{}, 1)
	c.cmdChan <- captureCommandDisable{ch}
	<-ch
}

// Rotate performs a rotation of the underlying flow log and
// returns an AggFlowMap with all flows that have been collected
// since the last call to Rotate(). It also returns capture statistics
// collected since the last call to Rotate().
//
// Note: stats.Pcap may be null if there was an error fetching the
// stats of the underlying pcap handle.
func (c *Capture) Rotate() (agg goDB.AggFlowMap, stats CaptureStats) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.closed {
		panic("Capture is closed")
	}

	ch := make(chan rotateResult, 1)
	c.cmdChan <- captureCommandRotate{ch}
	result := <-ch
	return result.agg, result.stats
}

// Close closes the Capture and releases all underlying resources.
// Close is idempotent. Once you have closed a Capture, you can no
// longer call any of its methods (apart from Close).
func (c *Capture) Close() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.closed {
		return
	}

	ch := make(chan struct{}, 1)
	c.cmdChan <- captureCommandDisable{ch}
	<-ch

	close(c.cmdChan)

	c.closed = true
}
