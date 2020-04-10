/////////////////////////////////////////////////////////////////////////////////
//
// capture.go
//
// Written by Lorenz Breidenbach lob@open.ch, December 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package capture

import (
	"fmt"
	"net"
	"sync"
	"syscall"
	"time"

	"github.com/els0r/goProbe/pkg/goDB"
	"github.com/els0r/log"
	"github.com/fako1024/gopacket"
	"github.com/fako1024/gopacket/layers"
	"github.com/fako1024/gopacket/pcap"
)

const (
	// Snaplen sets the amount of bytes captured from a packet
	Snaplen = 86

	// ErrorThreshold is the maximum amount of consecutive errors that can occur on an interface before capturing is halted.
	ErrorThreshold = 10000

	// CaptureTimeout sets the maximum duration pcap waits until polling the kernel for more packets. Our experiments show that you don't want to set this value lower
	// than roughly 100 ms. Otherwise we flood the kernel with syscalls
	// and our performance drops.
	CaptureTimeout time.Duration = 500 * time.Millisecond

	// MinPcapBufSize sets the minimum buffer size for capture on an interface
	MinPcapBufSize = 1024 // require at least one KiB
	// MaxPcapBufSize sets the maximum buffer size for capture on an interface.
	MaxPcapBufSize = 1024 * 1024 * 1024 // 1 GiB should be enough for anyone ;)
)

//////////////////////// Ancillary types ////////////////////////

// Config stores the parameters for capturing packets with libpcap
type Config struct {
	BufSize   int    `json:"buf_size"` // in bytes
	BPFFilter string `json:"bpf_filter"`
	Promisc   bool   `json:"promisc"`
}

// Validate (partially) checks that the given Config contains no bogus settings.
//
// Note that the BPFFilter field isn't checked.
func (cc Config) Validate() error {
	if !(MinPcapBufSize <= cc.BufSize && cc.BufSize <= MaxPcapBufSize) {
		return fmt.Errorf("invalid configuration entry BufSize. Value must be in range [%d, %d]", MinPcapBufSize, MaxPcapBufSize)
	}
	return nil
}

// State enumerates the activity states of a capture
type State byte

const (
	// StateUninitialized describes a capture that hasn't been set up (yet)
	StateUninitialized State = iota + 1
	// StateInitialized describes a capture that has been set up
	StateInitialized
	// StateActive means that the capture is actively capturing packets
	StateActive
	// StateError means that the capture has hit the error threshold on the interface (set by ErrorThreshold)
	StateError
)

func (cs State) String() string {
	switch cs {
	case StateUninitialized:
		return "StateUninitialized"
	case StateInitialized:
		return "StateInitialized"
	case StateActive:
		return "StateActive"
	case StateError:
		return "StateError"
	default:
		return "Unknown"
	}
}

// Stats stores the packet and pcap statistics of the capture
type Stats struct {
	Pcap          *pcap.Stats `json:"pcap"`
	PacketsLogged int         `json:"packets_logged"`
}

// Status stores both the capture's state and statistics
type Status struct {
	State State `json:"state"`
	Stats Stats `json:"stats"`
}

// ErrorMap stores all encountered pcap errors and their number of occurrence
type ErrorMap map[string]int

// String prints the errors that occurred during capturing
func (e ErrorMap) String() string {
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
	returnChan chan<- Status
}

type captureCommandErrors struct {
	returnChan chan<- ErrorMap
}

type captureCommandFlows struct {
	returnChan chan<- *FlowLog
}

func (cmd captureCommandStatus) execute(c *Capture) {
	var result Status

	result.State = c.state

	pcapStats := c.tryGetPcapStats()
	result.Stats = Stats{
		Pcap:          subPcapStats(pcapStats, c.lastRotationStats.Pcap),
		PacketsLogged: c.packetsLogged - c.lastRotationStats.PacketsLogged,
	}

	cmd.returnChan <- result
}

func (cmd captureCommandErrors) execute(c *Capture) {
	cmd.returnChan <- c.errMap
}

func (cmd captureCommandFlows) execute(c *Capture) {
	cmd.returnChan <- c.flowLog
}

type captureCommandUpdate struct {
	config     Config
	returnChan chan<- struct{}
}

func (cmd captureCommandUpdate) execute(c *Capture) {
	if c.state == StateActive {
		if c.needReinitialization(cmd.config) {
			c.deactivate()
		} else {
			cmd.returnChan <- struct{}{}
			return
		}
	}

	// Can no longer be in StateActive at this point
	// Now try to make Capture initialized with new config.
	switch c.state {
	case StateUninitialized:
		c.config = cmd.config
		c.initialize()
	case StateInitialized:
		if c.needReinitialization(cmd.config) {
			c.uninitialize()
			c.config = cmd.config
			c.initialize()
		}
	case StateError:
		c.recoverError()
		c.config = cmd.config
		c.initialize()
	}

	c.logger.Debugf("Interface '%s': (re)initialized for configuration update", c.iface)

	// If initialization in last step succeeded, activate
	if c.state == StateInitialized {
		c.activate()
	}

	cmd.returnChan <- struct{}{}
}

// helper struct to bundle up the multiple return values
// of Rotate
type rotateResult struct {
	agg   goDB.AggFlowMap
	stats Stats
}

type captureCommandRotate struct {
	returnChan chan<- rotateResult
}

func (cmd captureCommandRotate) execute(c *Capture) {
	var result rotateResult

	result.agg = c.flowLog.Rotate()

	pcapStats := c.tryGetPcapStats()

	result.stats = Stats{
		Pcap:          subPcapStats(pcapStats, c.lastRotationStats.Pcap),
		PacketsLogged: c.packetsLogged - c.lastRotationStats.PacketsLogged,
	}

	c.lastRotationStats = Stats{
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
	case StateUninitialized:
	case StateInitialized:
		c.uninitialize()
	case StateActive:
		c.deactivate()
		c.uninitialize()
	case StateError:
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

// PcapMutex linearizes all pcap.InactiveHandle.Activate and
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

	state State

	config Config

	// channel over which commands are passed to process()
	// close(cmdChan) is used to tell process() to stop
	cmdChan chan captureCommand

	// stats from the last rotation or reset (needed for Status)
	lastRotationStats Stats

	// Counts the total number of logged packets (since the creation of the
	// Capture)
	packetsLogged int

	// Logged flows since creation of the capture (note that some
	// flows are retained even after Rotate has been called)
	flowLog *FlowLog

	pcapHandle   *pcap.Handle
	packetSource *gopacket.PacketSource
	packet       GPPacket

	// error map for logging errors more properly
	errMap   ErrorMap
	errCount int

	// logging
	logger log.Logger
}

// NewCapture creates a new Capture associated with the given iface.
func NewCapture(iface string, config Config, logger log.Logger) *Capture {
	c := &Capture{
		iface:   iface,
		mutex:   sync.Mutex{},
		state:   StateUninitialized,
		config:  config,
		cmdChan: make(chan captureCommand, 1),
		lastRotationStats: Stats{
			Pcap:          &pcap.Stats{},
			PacketsLogged: 0,
		},
		flowLog: NewFlowLog(logger),
		errMap:  make(map[string]int),
		logger:  logger,
	}
	go c.process()
	return c
}

// setState provides write access to the state field of
// a Capture. It also logs the state change.
func (c *Capture) setState(s State) {
	c.state = s
	c.logger.Debugf("Interface '%s': entered capture state %s", c.iface, s)
}

// capturePacket peform the actual packet capture and handles errors gracefully
func (c *Capture) capturePacket() (err error) {

	// Grab the next gopacket from the source
	packet, err := c.packetSource.NextPacket()
	if err != nil {

		// Immediately retry for temporary network errors
		if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
			return nil
		}

		// Immediately retry for EAGAIN or CaptureTimeout expired
		if err == syscall.EAGAIN || err == pcap.NextErrorTimeoutExpired {
			return nil
		}

		// Return all other errors as breaking
		return fmt.Errorf("Capture error: %s", err)
	}

	// Populate "our" packet and handle any error
	if err := c.packet.Populate(packet); err != nil {
		c.errCount++

		// collect the error. The errors value is the key here. Otherwise, the address
		// of the error would be taken, which results in a non-minimal set of errors
		if _, exists := c.errMap[err.Error()]; !exists {
			// log the packet to the pcap error logs
			if logerr := PacketLog.Log(c.iface, packet, Snaplen); logerr != nil {
				c.logger.Info("failed to log faulty packet: " + logerr.Error())
			}
		}

		c.errMap[err.Error()]++

		// shut down the interface thread if too many consecutive decoding failures
		// have been encountered
		if c.errCount > ErrorThreshold {
			return fmt.Errorf("The last %d packets could not be decoded: [%s ]",
				ErrorThreshold,
				c.errMap.String(),
			)
		}
	}

	c.flowLog.Add(&c.packet)
	c.errCount = 0
	c.packetsLogged++

	return nil
}

// process is the heart of the Capture. It listens for network traffic on the
// network interface and logs the corresponding flows.
//
// As long as the Capture is in StateActive process() is capturing
// packets from the network. In any other state, process() only awaits
// further commands.
//
// process keeps running its own goroutine until Close is called on its Capture.
func (c *Capture) process() {
	for {
		select {
		case cmd, ok := <-c.cmdChan:
			if ok {
				cmd.execute(c)
			} else {
				return
			}
		default:
			if c.state == StateActive {
				if err := c.capturePacket(); err != nil {
					c.setState(StateError)
					c.logger.Errorf("Interface '%s': %s", c.iface, err.Error())
				}
			}
		}
	}
}

//////////////////////// state transisition functions ////////////////////////

// initialize attempts to transition from StateUninitialized
// into StateInitialized. If an error occurrs, it instead
// transitions into state StateError.
func (c *Capture) initialize() {
	initializationErr := func(msg string, args ...interface{}) {
		c.logger.Errorf(msg, args...)
		c.setState(StateError)
		return
	}

	if c.state != StateUninitialized {
		panic("Need state StateUninitialized")
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
	c.packetSource.DecodeOptions = gopacket.DecodeOptions{
		Lazy:               true,
		NoCopy:             true,
		SkipDecodeRecovery: false,
	}

	c.setState(StateInitialized)
}

// uninitialize moves from StateInitialized to StateUninitialized.
func (c *Capture) uninitialize() {
	if c.state != StateInitialized {
		panic("Need state StateInitialized")
	}
	c.reset()
}

// activate transitions from StateInitialized
// into StateActive.
func (c *Capture) activate() {
	if c.state != StateInitialized {
		panic("Need state StateInitialized")
	}
	c.setState(StateActive)
	c.logger.Debugf("Interface '%s': capture active. Link type: %s", c.iface, c.pcapHandle.LinkType())
}

// deactivate transitions from StateActive
// into StateInitialized.
func (c *Capture) deactivate() {
	if c.state != StateActive {
		panic("Need state StateActive")
	}
	c.setState(StateInitialized)
	c.logger.Debugf("Interface '%s': deactivated", c.iface)
}

// recoverError transitions from StateError
// into StateUninitialized
func (c *Capture) recoverError() {
	if c.state != StateError {
		panic("Need state StateError")
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
	c.setState(StateUninitialized)

	// reset the error map. The GC will take care of the previous
	// one
	c.errMap = make(map[string]int)
}

// needReinitialization checks whether we need to reinitialize the capture
// to apply the given config.
func (c *Capture) needReinitialization(config Config) bool {
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
			c.logger.Errorf("Interface '%s': error while requesting pcap stats: %s", err.Error())
		}
	}
	return pcapStats
}

// subPcapStats computes a - b (fieldwise) if both a and b
// are not nil. Otherwise, it returns nil.
func subPcapStats(a, b *pcap.Stats) *pcap.Stats {
	if a == nil || b == nil {
		return nil
	}
	return &pcap.Stats{
		PacketsReceived:  a.PacketsReceived - b.PacketsReceived,
		PacketsDropped:   a.PacketsDropped - b.PacketsDropped,
		PacketsIfDropped: a.PacketsIfDropped - b.PacketsIfDropped,
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
	if err := inactive.SetSnapLen(int(Snaplen)); err != nil {
		inactive.CleanUp()
		return nil, err
	}

	// set promisc mode
	if err := inactive.SetPromisc(promisc); err != nil {
		inactive.CleanUp()
		return nil, err
	}

	// set timeout
	if err := inactive.SetTimeout(CaptureTimeout); err != nil {
		inactive.CleanUp()
		return nil, err
	}

	// return the inactive handle for activation
	return inactive, err
}

//////////////////////// public functions ////////////////////////

// Status returns the current State as well as the statistics
// collected since the last call to Rotate()
//
// Note: If the Capture was reinitialized since the last rotation,
// result.Stats.Pcap will be inaccurate.
//
// Note: result.Stats.Pcap may be null if there was an error fetching the
// stats of the underlying pcap handle.
func (c *Capture) Status() (result Status) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.closed {
		panic("Capture is closed")
	}

	ch := make(chan Status, 1)
	c.cmdChan <- captureCommandStatus{ch}
	return <-ch
}

// Errors implements the status call to return all interface errors
func (c *Capture) Errors() (result ErrorMap) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.closed {
		panic("Capture is closed")
	}

	ch := make(chan ErrorMap, 1)
	c.cmdChan <- captureCommandErrors{ch}
	return <-ch
}

// Flows impolements the status call to return the contents of the active flow log
func (c *Capture) Flows() (result *FlowLog) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.closed {
		panic("Capture is closed")
	}

	ch := make(chan *FlowLog, 1)
	c.cmdChan <- captureCommandFlows{ch}
	return <-ch
}

// Update will attempt to put the Capture instance into
// StateActive with the given config.
// If the Capture is already active with the given config
// Update will detect this and do no work.
func (c *Capture) Update(config Config) {
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
// StateActive.
// Enable will have no effect if the Capture is already
// in StateActive.
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

// Disable will bring the Capture instance into StateUninitialized
// Disable will have no effect if the Capture is already
// in StateUninitialized.
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
func (c *Capture) Rotate() (agg goDB.AggFlowMap, stats Stats) {
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
