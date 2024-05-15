package server

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/els0r/telemetry/logging"
)

// OpenAPISpecWriter can be implemented by any server able to produce an OpenAPI spec
// from its registered routes
type OpenAPISpecWriter interface {
	WriteOpenAPISpec(w io.Writer) error
}

// GenerateSpec writes the OpenAPI spec to a file
func GenerateSpec(ctx context.Context, path string, ow OpenAPISpecWriter) error {
	if path == "" {
		return nil
	}
	logger := logging.FromContext(ctx).With("path", path)

	logger.Info("writing OpenAPI spec only")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}

	err = ow.WriteOpenAPISpec(f)
	if err != nil {
		return fmt.Errorf("failed to write OpenAPI spec to %s: %w", path, err)
	}
	return nil
}
