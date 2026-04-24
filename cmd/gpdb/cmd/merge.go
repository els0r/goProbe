package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/els0r/goProbe/v4/pkg/goDB"
	"github.com/spf13/cobra"
)

const (
	flagInterface         = "iface"
	flagOverwrite         = "overwrite"
	flagDryRun            = "dry-run"
	flagCompleteTolerance = "complete-tolerance"
)

type mergeCommandOptions struct {
	interfaces        []string
	overwrite         bool
	dryRun            bool
	completeTolerance time.Duration
}

func newMergeCmd() *cobra.Command {
	opts := mergeCommandOptions{}

	mergeCmd := &cobra.Command{
		Use:   "merge SOURCE_DB DESTINATION_DB",
		Short: "Merge source goDB into destination goDB",
		Long: `Merge source goDB into destination goDB.

Days that are complete can be copied directly when safe, while partial-day
data is rebuilt block-by-block to ensure metadata is re-encoded in the
destination database layout.`,
		Args: cobra.ExactArgs(2),
		RunE: wrapCancellationContext(func(ctx context.Context, cmd *cobra.Command, args []string) error {
			return mergeEntrypoint(ctx, cmd, args, opts)
		}),
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	mergeFlags := mergeCmd.Flags()
	mergeFlags.StringSliceVar(&opts.interfaces, flagInterface, nil, "interface(s) to merge (default: all interfaces found in source)")
	mergeFlags.BoolVar(&opts.overwrite, flagOverwrite, false, "prefer source on conflicts (for complete-day collisions, replace destination day)")
	mergeFlags.BoolVar(&opts.dryRun, flagDryRun, false, "show planned actions without mutating destination")
	mergeFlags.DurationVar(&opts.completeTolerance, flagCompleteTolerance, 150*time.Second, "tolerance for classifying full-day coverage")

	return mergeCmd
}

func mergeEntrypoint(ctx context.Context, cmd *cobra.Command, args []string, opts mergeCommandOptions) error {
	summary, err := goDB.MergeDatabases(ctx, goDB.MergeOptions{
		SourcePath:        args[0],
		DestinationPath:   args[1],
		Interfaces:        opts.interfaces,
		Overwrite:         opts.overwrite,
		DryRun:            opts.dryRun,
		CompleteTolerance: opts.completeTolerance,
	})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			cmd.SilenceUsage = true
		}
		return fmt.Errorf("failed to merge source DB `%s` into destination DB `%s`: %w", args[0], args[1], err)
	}

	fmt.Printf("Merge completed (dry-run=%t)\n", summary.DryRun)
	fmt.Printf("Interfaces processed: %d\n", summary.InterfacesProcessed)
	fmt.Printf("Days copied: %d\n", summary.DaysCopied)
	fmt.Printf("Days rebuilt: %d\n", summary.DaysRebuilt)
	fmt.Printf("Days skipped: %d\n", summary.DaysSkipped)
	fmt.Printf("Conflicts resolved by destination: %d\n", summary.ConflictsResolvedByDestination)
	fmt.Printf("Conflicts resolved by source: %d\n", summary.ConflictsResolvedBySource)

	return nil
}
