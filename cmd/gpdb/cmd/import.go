package cmd

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/els0r/goProbe/v4/cmd/gpdb/pkg/csvimport"
	"github.com/els0r/goProbe/v4/pkg/goDB/encoder/encoders"
	"github.com/spf13/cobra"
)

const (
	flagSchema      = "schema"
	flagMaxRows     = "max-rows"
	flagEncoder     = "encoder"
	flagPermissions = "permissions"
)

type importCommandOptions struct {
	schema      string
	iface       string
	maxRows     int
	encoder     string
	permissions string
}

func newImportCmd() *cobra.Command {
	opts := importCommandOptions{}

	importCmd := &cobra.Command{
		Use:   "import SOURCE_CSV DESTINATION_DB",
		Short: "Import flow rows from CSV into goDB",
		Long: `Import flow rows from CSV into goDB.

When no schema is provided, the first CSV row is treated as a header schema.
The schema must include a time field and either an iface field or --iface.`,
		Args: cobra.ExactArgs(2),
		RunE: wrapCancellationContext(func(ctx context.Context, cmd *cobra.Command, args []string) error {
			return importEntrypoint(ctx, cmd, args, opts)
		}),
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	flags := importCmd.Flags()
	flags.StringVar(&opts.schema, flagSchema, "", "CSV schema (comma-separated fields); defaults to first CSV row")
	flags.StringVar(&opts.iface, flagInterface, "", "interface for all rows when schema has no iface field")
	flags.IntVarP(&opts.maxRows, flagMaxRows, "n", 0, "maximum number of CSV data rows to import (0 = all)")
	flags.StringVar(&opts.encoder, flagEncoder, "lz4", "compression encoder (lz4, zstd, null, lz4cust or numeric type)")
	flags.StringVar(&opts.permissions, flagPermissions, "", "file permissions mode for created DB files/directories (e.g. 0644)")

	return importCmd
}

func importEntrypoint(ctx context.Context, cmd *cobra.Command, args []string, opts importCommandOptions) error {
	encoderType, err := parseEncoderType(opts.encoder)
	if err != nil {
		return err
	}

	permissions, err := csvimport.ParsePermissions(opts.permissions)
	if err != nil {
		return err
	}

	summary, err := csvimport.Import(ctx, csvimport.Options{
		InputPath:   args[0],
		OutputPath:  args[1],
		Schema:      opts.schema,
		Interface:   opts.iface,
		MaxRows:     opts.maxRows,
		EncoderType: encoderType,
		Permissions: permissions,
	})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			cmd.SilenceUsage = true
		}
		return fmt.Errorf("failed to import CSV `%s` into DB `%s`: %w", args[0], args[1], err)
	}

	fmt.Printf("Import completed\n")
	fmt.Printf("Rows read: %d\n", summary.RowsRead)
	fmt.Printf("Rows imported: %d\n", summary.RowsImported)
	fmt.Printf("Rows skipped: %d\n", summary.RowsSkipped)
	fmt.Printf("Interfaces written: %d\n", summary.Interfaces)
	fmt.Printf("Blocks written: %d\n", summary.BlocksWritten)

	return nil
}

func parseEncoderType(raw string) (encoders.Type, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return encoders.EncoderTypeLZ4, nil
	}

	if parsed, err := strconv.ParseUint(raw, 10, 8); err == nil {
		encoderType := encoders.Type(parsed)
		if encoderType > encoders.MaxEncoderType {
			return 0, fmt.Errorf("encoder type `%d` is out of range", parsed)
		}
		return encoderType, nil
	}

	encoderType, err := encoders.GetTypeByString(raw)
	if err != nil {
		return 0, err
	}

	return encoderType, nil
}
