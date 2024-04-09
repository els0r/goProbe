package capturetypes

// ParsingErrno denotes a non-critical packet parsing error / failure
type ParsingErrno int8

const (
	// ErrnoOK : No Error
	ErrnoOK ParsingErrno = iota - 1

	// ErrnoPacketFragmentIgnore : packet fragment does not carry relevant information
	// (will be skipped as non-error)
	ErrnoPacketFragmentIgnore

	// ErrnoInvalidIPHeader : received neither IPv4 nor IPv6 IP header
	ErrnoInvalidIPHeader

	// ErrnoPacketTruncated : packet too short / truncated
	ErrnoPacketTruncated

	// NumParsingErrors : Number of tracked parsing errors
	NumParsingErrors
)

// ParsingErrnoNames maps a ParsingErrno to a string
var ParsingErrnoNames = [NumParsingErrors]string{
	"packet fragmented",
	"invalid IP header",
	"packet truncated",
}

// String returns a string representation of the underlying ParsingErrno
func (e ParsingErrno) String() string {
	return ParsingErrnoNames[e]
}

// ParsingFailed denotes if a ParsingErrno actually signifies that packet parsing failed
func (e ParsingErrno) ParsingFailed() bool {
	return e >= ErrnoInvalidIPHeader
}

// ParsingErrTracker denotes a simple table-based parsing error structure for counting
// all available parsing error (errno) types
type ParsingErrTracker [NumParsingErrors]int

// Sum returns the sum of all errors (inclunding non-critical ones) currently tracked
// in the error table
func (e *ParsingErrTracker) Sum() (res int) {
	for i := ErrnoPacketFragmentIgnore; i < NumParsingErrors; i++ {
		res += e[i]
	}
	return
}

// SumFailed returns the sum of all errors (that prevent packet processing) currently tracked
// in the error table
func (e *ParsingErrTracker) SumFailed() (res int) {
	for i := ErrnoInvalidIPHeader; i < NumParsingErrors; i++ {
		res += e[i]
	}
	return
}

// Reset resets all error counters in the error table (for reuse)
func (e *ParsingErrTracker) Reset() {
	for i := ErrnoPacketFragmentIgnore; i < NumParsingErrors; i++ {
		e[i] = 0
	}
}
