package main

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/charmbracelet/lipgloss"
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
			if err := initLogger(cmd.Verbose); err != nil {
				return err
			}

			dataDir := getDataDir()
			dbPath := filepath.Join(dataDir, "bsky-browser.db")
			if err := Open(dbPath); err != nil {
				return fmt.Errorf("failed to open database: %w", err)
			}

			return nil
		},
		RunE: func(c *cobra.Command, args []string) error {
			return cmd.Run(c)
		},
	}

	root.Flags().BoolVarP(&cmd.Verbose, "verbose", "v", false, "Enable verbose logging")

	root.AddCommand(newLoginCommand())
	root.AddCommand(newWhoamiCommand())
	root.AddCommand(newRefreshCommand())

	return root
}

func (cmd *RootCmd) Run(c *cobra.Command) error {
	logger.Info("bsky-browser started")
	logger.Debug("verbose mode active", "verbose", cmd.Verbose)
	fmt.Println("bsky-browser - Run with --help for usage information")
	return nil
}

func newLoginCommand() *cobra.Command {
	var handle string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to Bluesky via OAuth",
		Long:  "Opens a browser to authenticate with Bluesky using OAuth. After successful authentication, your tokens will be stored locally.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if handle == "" {
				fmt.Print("Enter your Bluesky handle (e.g., alice.bsky.social): ")
				if _, err := fmt.Scanln(&handle); err != nil {
					return fmt.Errorf("failed to read handle: %w", err)
				}
			}

			am := NewAuthManager()
			if err := am.Login(context.Background(), handle); err != nil {
				return fmt.Errorf("login failed: %w", err)
			}

			fmt.Println("✓ Successfully logged in!")
			return nil
		},
	}

	cmd.Flags().StringVar(&handle, "handle", "", "Bluesky handle to login with")

	return cmd
}

func newWhoamiCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Display current user information",
		Long:  "Shows the handle and DID of the currently authenticated user. The handle is resolved from the DID via an API call and cached. Use --force to refresh the cached handle.",
		RunE: func(cmd *cobra.Command, args []string) error {
			am := NewAuthManager()
			auth, err := am.Whoami(force)
			if err != nil {
				return err
			}

			style := lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#7D56F4"))

			fmt.Printf("%s %s\n", style.Render("Username:"), auth.Handle)
			fmt.Printf("%s %s\n", style.Render("DID:"), auth.DID)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force refresh of handle resolution")

	return cmd
}

func newRefreshCommand() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:     "refresh",
		Aliases: []string{"index"},
		Short:   "Fetch and index all bookmarks and likes",
		Long:    "Fetches all your saved bookmarks and liked posts from Bluesky and indexes them into the local SQLite database for offline searching.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			client, err := NewBlueskyClient(ctx)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			if err := client.RefreshAndIndex(ctx, limit); err != nil {
				return fmt.Errorf("failed to refresh and index: %w", err)
			}

			fmt.Println("✓ Successfully indexed all bookmarks and likes!")
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 0, "Limit the number of posts to fetch (0 = no limit)")

	return cmd
}
