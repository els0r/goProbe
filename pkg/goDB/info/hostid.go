package info

import (
	"github.com/denisbrodbeck/machineid"
)

// GetHostID is a method that returns a system's unique identifier
func GetHostID() string {
	id, err := machineid.ID()
	if err != nil {
		return ""
	}
	return id
}
