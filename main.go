// Application entrypoint
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/fang"
	"github.com/charmbracelet/log"
)

func main() {
	rootCmd := NewRootCommand()
	fang.Execute(context.Background(), rootCmd)
}

func initLogger(verbose bool) error {
	logLevel := log.InfoLevel
	if verbose {
		logLevel = log.DebugLevel
	}

	l, err := NewLogger(getLogPath(), logLevel)
	if err != nil {
		return err
	}

	logger = l
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
		configDir = filepath.Join(home, ".config")
	}

	appDir := filepath.Join(configDir, "bsky-browser", "logs")
	os.MkdirAll(appDir, 0755)

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	return filepath.Join(appDir, fmt.Sprintf("bsky-browser_%s.log", timestamp))
}
