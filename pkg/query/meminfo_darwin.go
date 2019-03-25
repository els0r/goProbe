//+build darwin

package query

const gb = 1204 * 1024 * 1024

func getPhysMem() (float64, error) {
	// TODO: proper way to extract the available physical memory
	// Defaulting to 4 GB to say on the safe side
	var physMem = float64(4 * gb)

	return physMem, nil
}
