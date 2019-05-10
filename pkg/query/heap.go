package query

import (
	"fmt"
	"runtime"
	"time"
)

// Parameters for checking memory consumption of query
const (
	MemCheckInterval = 1 * time.Second
)

// watchHeap makes sure to alert on too high memory consumption
func (s *Statement) watchHeap(errors chan error) chan struct{} {
	stopChan := make(chan struct{})

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

		memTicker := time.NewTicker(MemCheckInterval)
		m := runtime.MemStats{}
		for {
			select {
			case <-memTicker.C:
				runtime.ReadMemStats(&m)

				// Check if current memory consumption is higher than maximum allowed percentage of the available
				// physical memory
				if (m.Sys-m.HeapReleased)/1024 > uint64(float64(s.MaxMemPct)*physMem/100) {
					memTicker.Stop()
					errors <- fmt.Errorf("memory consumption above %v%% of physical memory. Aborting query", s.MaxMemPct)
					return
				}
			case <-stopChan:
				memTicker.Stop()
				return
			}
		}
	}()
	return stopChan
}
