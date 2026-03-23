package main

import (
	"log/slog"

	"github.com/els0r/goProbe/v4/cmd/goProbe/cmd"
	"github.com/els0r/telemetry/logging"
)

func main() {
	err := cmd.Execute()
	if err != nil {
		logger, _, _ := logging.New(slog.LevelInfo, "logfmt")
		logger.Fatal("goProbe terminated with an error", "error", err)
	}
}
