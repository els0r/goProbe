package workload

import (
	"log/slog"
	"sync"

	"github.com/els0r/goProbe/v4/pkg/goDB/storage/gpfile"
)

// Workload stores all relevant parameters to load a block and execute a query on it
type Workload struct {
	workDirs []*gpfile.GPDir

	stats      *Stats
	statsFuncs StatsFuncs
}

// New creates a new workload
func New(workDirs []*gpfile.GPDir) *Workload {
	return &Workload{
		workDirs: workDirs,
		stats:    &Stats{Workloads: 1},
	}
}

// WorkDirs returns the directories the workload is working on
func (w *Workload) WorkDirs() []*gpfile.GPDir {
	return w.workDirs
}

// AddStats adds processing statistics to the stats of the workload
func (w *Workload) AddStats(s *Stats) {
	w.stats.Add(s)
}

// Stats returns the stats tracked by the workload
func (w *Workload) Stats() *Stats {
	return w.stats
}

// AppendStatsFunc appends the stats function to the existing stats func of the workload
func (w *Workload) AppendStatsFunc(fn StatsFunc) StatsFuncs {
	return w.statsFuncs.Append(fn)
}

// StatsFunc is a function which can be used to communicate running workload statistics
type StatsFunc func(stats *Stats)

// StatsCallbaacks is a list of StatsFuncs
type StatsFuncs []StatsFunc

// Append appends a stats func to the existing stats funcs
func (sfc StatsFuncs) Append(fn StatsFunc) StatsFuncs {
	return append(sfc, fn)
}

// Execute runs all callbacks
func (scf StatsFuncs) Execute(stats *Stats) {
	for _, fn := range scf {
		fn(stats)
	}
}

// Stats tracks interactions with the underlying DB data
type Stats struct {
	sync.RWMutex

	BytesLoaded          uint64 `json:"bytes_loaded" doc:"Bytes loaded from disk"`
	BytesDecompressed    uint64 `json:"bytes_decompressed" doc:"Effective block size after decompression"`
	BlocksProcessed      uint64 `json:"blocks_processed" doc:"Number of blocks loaded from disk"`
	BlocksCorrupted      uint64 `json:"blocks_corrupted" doc:"Blocks which could not be loaded or processed"`
	DirectoriesProcessed uint64 `json:"directories_processed" doc:"Number of directories processed"`
	Workloads            uint64 `json:"workloads" doc:"Total number of workloads to be processed"`
}

// LogValue implements the slog.LogValuer interface
func (s *Stats) LogValue() (v slog.Value) {
	v = slog.GroupValue(
		slog.Uint64("bytes_loaded", s.BytesLoaded),
		slog.Uint64("bytes_decompressed", s.BytesDecompressed),
		slog.Uint64("blocks_processed", s.BlocksProcessed),
		slog.Uint64("blocks_corrupted", s.BlocksCorrupted),
		slog.Uint64("directories_processed", s.DirectoriesProcessed),
		slog.Uint64("workloads", s.Workloads),
	)
	return
}

// Add adds the values of stats to s
func (s *Stats) Add(stats *Stats) {
	if stats == nil {
		return
	}
	s.Lock()
	stats.RLock()
	s.BlocksProcessed += stats.BlocksProcessed
	s.BytesDecompressed += stats.BytesDecompressed
	s.BlocksProcessed += stats.BlocksProcessed
	s.BlocksCorrupted += stats.BlocksCorrupted
	s.DirectoriesProcessed += stats.DirectoriesProcessed
	s.Workloads += stats.Workloads
	stats.RUnlock()
	s.Unlock()
}
