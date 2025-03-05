package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"

	"github.com/els0r/goProbe/v4/cmd/goQuery/pkg/conf"
	"github.com/els0r/telemetry/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func initProfiling(_ *cobra.Command, _ []string) error {
	// Setup profiling (if enabled)
	profilingOutputDir := viper.GetString(conf.ProfilingOutputDir)
	if profilingOutputDir != "" {
		logging.Logger().With(conf.ProfilingOutputDir, profilingOutputDir).Debug("setting up profiling")
		if err := startProfiling(profilingOutputDir); err != nil {
			return fmt.Errorf("failed to initialize profiling: %w", err)
		}
	}
	return nil
}

func finishProfiling(_ *cobra.Command, _ []string) error {
	profilingOutputDir := viper.GetString(conf.ProfilingOutputDir)
	if profilingOutputDir != "" {
		if err := stopProfiling(profilingOutputDir); err != nil {
			return fmt.Errorf("failed to finalize profiling: %w", err)
		}
	}
	return nil
}

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
