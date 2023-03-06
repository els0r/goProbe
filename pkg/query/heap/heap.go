package heap

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"runtime/debug"
	"time"
)

// Parameters for checking memory consumption of query
const (
	MemCheckInterval = 1 * time.Second

	// Variables for manual garbage collection calls
	goGCInterval = 5 * time.Second
	goGCLimit    = 6291456 // Limit for GC call, in bytes
)

var (
	ErrorMemoryBreach = errors.New("maximum memory breach")
)

// Watch makes sure to alert on too high memory consumption
func Watch(ctx context.Context, maxAllowedMemPct int) (errors chan error) {
	errors = make(chan error)
	go func() {
		// obtain physical memory of this host
		var (
			physMem float64
			err     error
		)

		physMem, err = getPhysMem()
		if err != nil {
			errors <- err
			return
		}

		// Create global MemStats object for tracking of memory consumption
		memTicker := time.NewTicker(MemCheckInterval)
		m := runtime.MemStats{}
		lastGC := time.Now()

		for {
			select {
			case <-memTicker.C:
				runtime.ReadMemStats(&m)

				usedMem := m.Sys - m.HeapReleased
				maxAllowedMem := uint64(float64(maxAllowedMemPct) * physMem / 100)

				// Check if current memory consumption is higher than maximum allowed percentage of the available
				// physical memory
				if usedMem/1024 > maxAllowedMem {
					memTicker.Stop()
					errors <- fmt.Errorf("%w: %v%% of physical memory", ErrorMemoryBreach, maxAllowedMemPct)
					return
				}

				// Conditionally call a manual garbage collection and memory release if the current heap allocation
				// is above goGCLimit and more than goGCInterval seconds have passed
				if usedMem > goGCLimit && time.Since(lastGC) > goGCInterval {
					runtime.GC()
					debug.FreeOSMemory()
					lastGC = time.Now()
				}
			case <-ctx.Done():
				memTicker.Stop()
				return
			}
		}
	}()
	return errors
}
