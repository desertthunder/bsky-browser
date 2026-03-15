// Logging setup
package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/charmbracelet/log"
)

var logger *log.Logger

// initLogger initializes the logger based on verbosity level:
//
//	0 = file only (no stderr output)
//	1 = file + stderr (info level)
//	2+ = file + stderr (debug level)
func initLogger(verbosity int) error {
	logPath := getLogPath()

	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	var writers []io.Writer
	writers = append(writers, file)

	var level log.Level
	switch verbosity {
	case 0:
		level = log.InfoLevel
	case 1:
		writers = append(writers, os.Stderr)
		level = log.InfoLevel
	case 2, 3:
		writers = append(writers, os.Stderr)
		level = log.DebugLevel
	default:
		writers = append(writers, os.Stderr)
		level = log.DebugLevel
	}

	var w io.Writer
	if len(writers) == 1 {
		w = writers[0]
	} else {
		w = io.MultiWriter(writers...)
	}

	logger = log.NewWithOptions(w, log.Options{
		Level:           level,
		ReportTimestamp: true,
		ReportCaller:    true,
		TimeFormat:      time.Kitchen,
		Prefix:          "[BSKY-BROWSER]",
	})

	return nil
}

func getLogPath() string {
	if path := os.Getenv("BSKY_BROWSER_LOG"); path != "" {
		return path
	}

	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "/tmp/bsky-browser.log"
		}
		configDir = home + "/.config"
	}

	appDir := configDir + "/bsky-browser/logs"
	os.MkdirAll(appDir, 0755)

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	return appDir + fmt.Sprintf("/bsky-browser_%s.log", timestamp)
}
