package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
)

func startProfiling(dirPath string) error {

	err := os.MkdirAll(dirPath, 0750)
	if err != nil {
		return fmt.Errorf("failed to create pprof directory: %w", err)
	}

	f, err := os.Create(filepath.Join(filepath.Clean(dirPath), "goquery_cpu_profile.pprof"))
	if err != nil {
		return fmt.Errorf("failed to create CPU profile file: %w", err)
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		return fmt.Errorf("failed to start CPU profiling: %w", err)
	}

	return nil
}

func stopProfiling(dirPath string) error {

	pprof.StopCPUProfile()

	f, err := os.Create(filepath.Join(filepath.Clean(dirPath), "goquery_mem_profile.pprof"))
	if err != nil {
		return fmt.Errorf("failed to create memory profile file: %w", err)
	}
	if err := pprof.Lookup("allocs").WriteTo(f, 0); err != nil {
		return fmt.Errorf("failed to start memory profiling: %w", err)
	}

	return nil
}
