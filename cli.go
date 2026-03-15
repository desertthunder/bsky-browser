package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

type RootCmd struct {
	Verbose bool
}

func NewRootCommand() *cobra.Command {
	cmd := &RootCmd{}

	root := &cobra.Command{
		Use:   "bsky-browser",
		Short: "A CLI tool for browsing Bluesky bookmarks and likes",
		Long:  "bsky-browser is a CLI tool that allows you to search and browse your Bluesky bookmarks and likes.",
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
			return initLogger(cmd.Verbose)
		},
		RunE: func(c *cobra.Command, args []string) error {
			return cmd.Run(c)
		},
	}

	root.Flags().BoolVarP(&cmd.Verbose, "verbose", "v", false, "Enable verbose logging")

	return root
}

func (cmd *RootCmd) Run(c *cobra.Command) error {
	logger.Info("bsky-browser started")
	logger.Debug("verbose mode active", "verbose", cmd.Verbose)
	fmt.Println("bsky-browser - Run with --help for usage information")
	return nil
}
