package main

import (
	"fmt"
	"os"

	"github.com/els0r/goProbe/v4/cmd/global-query/cmd"
	"github.com/els0r/telemetry/logging"
)

func main() {
	err := cmd.Execute()
	if err != nil {
		logger, logErr := logging.New(logging.LevelError, logging.EncodingPlain,
			logging.WithOutput(os.Stderr),
		)
		if logErr != nil {
			fmt.Fprintf(os.Stderr, "Failed to instantiate CLI logger: %v\n", logErr)

			fmt.Fprintf(os.Stderr, "Error running application: %s\n", err)
			os.Exit(1)
		}
		logger.Fatalf("Error running application: %s", err)
	}
}
