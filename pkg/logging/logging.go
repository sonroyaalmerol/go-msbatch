package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

func NewLoggerFromEnv() *slog.Logger {
	debug := os.Getenv("MSBATCH_DEBUG")
	if debug == "" || debug == "0" || strings.ToLower(debug) == "false" || strings.ToLower(debug) == "off" {
		// Return a logger that discards everything
		return slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	output := os.Getenv("MSBATCH_DEBUG_FILE")
	var writer io.Writer = os.Stderr

	switch strings.ToLower(output) {
	case "stdout":
		writer = os.Stdout
	case "stderr", "":
		writer = os.Stderr
	default:
		f, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			writer = f
		}
	}

	return slog.New(slog.NewTextHandler(writer, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}
