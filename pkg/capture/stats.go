package capture

type CaptureStats struct {
	PacketsReceived int
	PacketsDropped  int
}

// add is a convenience method to total capture stats. This is relevant in the scope of
// adding statistics from the two directions. The result of the addition is written back
// to a to reduce allocations
func add(a, b *CaptureStats) {
	if a == nil || b == nil {
		return
	}
	a.PacketsReceived += b.PacketsReceived
	a.PacketsDropped += b.PacketsDropped
}

// sub is a convenience method to total capture stats. This is relevant in the scope of
// subtracting statistics from the two directions. The result of the subtraction is written back
// to a to reduce allocations
func sub(a, b *CaptureStats) {
	if a == nil || b == nil {
		return
	}
	a.PacketsReceived -= b.PacketsReceived
	a.PacketsDropped -= b.PacketsDropped
}
