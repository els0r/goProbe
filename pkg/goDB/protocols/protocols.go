/*
Package protocols provides lookup functionality for IP protocol IDs and their names (which are
in some cases OS specific)
*/
package protocols

//go:generate go run protocols_generator.go

// GetIPProto returns the friendly name for a given protocol id
func GetIPProto(id int) string {
	return IPProtocols[id]
}

// GetIPProtoID returns the numeric value for a given IP protocol
func GetIPProtoID(name string) (uint64, bool) {
	ret, ok := IPProtocolIDs[name]
	return uint64(ret), ok
}
