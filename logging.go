// Logging setup
package main

import (
	"io"
	"os"
	"time"

	"github.com/charmbracelet/log"
)

var logger *log.Logger

func NewLogger(fp string, level log.Level) (*log.Logger, error) {
	file, err := os.OpenFile(fp, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	w := io.MultiWriter(file, os.Stderr)

	return log.NewWithOptions(w, log.Options{
		Level:           level,
		ReportTimestamp: true,
		ReportCaller:    true,
		TimeFormat:      time.Kitchen,
		Prefix:          "[BSKY-BROWSER]",
	}), nil
}
