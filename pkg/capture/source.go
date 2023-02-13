package capture

type CaptureStats struct {
	PacketsReceived  int
	PacketsDropped   int
	PacketsIfDropped int
}

// add is a convenience method to total capture stats. This is relevant in the scope of
// adding statistics from the two directions. The result of the addition is written back
// to a to reduce allocations
func add(a, b *CaptureStats) {
	a.PacketsReceived += b.PacketsReceived
	a.PacketsDropped += b.PacketsDropped
	a.PacketsIfDropped += b.PacketsIfDropped
}
