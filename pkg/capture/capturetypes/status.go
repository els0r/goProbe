package capturetypes

// State enumerates the activity states of a capture
type State byte

const (
	// StateInitializing means that the capture is setting up
	StateInitializing State = iota + 1
	// StateCapturing means that the capture is actively capturing packets
	StateCapturing
	// StateClose means that the capture is fully terminating and it's held resources are
	// cleaned up
	StateClosing
	// StateError means that the capture has hit the error threshold on the interface (set by ErrorThreshold)
	StateError
)

func (cs State) String() string {
	switch cs {
	case StateInitializing:
		return "initializing"
	case StateCapturing:
		return "capturing"
	case StateClosing:
		return "closing"
	case StateError:
		return "inError"
	default:
		return "unknown"
	}
}

// Stats stores the packet statistics of the capture
type PacketStats struct {
	*CaptureStats
}

// Status stores both the capture's state and statistics
type InterfaceStatus struct {
	State       State       `json:"state"`
	PacketStats PacketStats `json:"packet_stats"`
}

type CaptureStats struct {
	Received int `json:"received"`
	Dropped  int `json:"dropped"`
}

// add is a convenience method to total capture stats. This is relevant in the scope of
// adding statistics from the two directions. The result of the addition is written back
// to a to reduce allocations
func add(a, b *CaptureStats) {
	if a == nil || b == nil {
		return
	}
	a.Received += b.Received
	a.Dropped += b.Dropped
}

// sub is a convenience method to total capture stats. This is relevant in the scope of
// subtracting statistics from the two directions. The result of the subtraction is written back
// to a to reduce allocations
func Sub(a, b *CaptureStats) {
	if a == nil || b == nil {
		return
	}
	a.Received -= b.Received
	a.Dropped -= b.Dropped
}
