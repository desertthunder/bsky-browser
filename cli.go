package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// RootCmd holds root command flags
type RootCmd struct {
	Verbose int
}

// SearchCmd holds search-specific flags
type SearchCmd struct {
	Query string
	Saved bool
	Liked bool
	Force bool
	Limit int
}

func rootCmd() *cobra.Command {
	cmd := &RootCmd{}
	searchCmd := &SearchCmd{}

	root := &cobra.Command{
		Use:   "bsky-browser [QUERY]",
		Short: "A CLI tool for browsing Bluesky bookmarks and likes",
		Long: `bsky-browser is a CLI tool that allows you to search and browse your Bluesky bookmarks and likes.

Examples:
  bsky-browser "golang"              # Search all posts for "golang"
  bsky-browser "query" --saved       # Search only saved/bookmarked posts
  bsky-browser "query" --liked       # Search only liked posts
  bsky-browser -q "query" -f         # Force re-index before searching`,
		Args: cobra.ArbitraryArgs,
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
			return searchCmd.Run(c, args)
		},
	}

	// Root flags - use CountVarP to support -v and -vv
	root.Flags().CountVarP(&cmd.Verbose, "verbose", "v", "Enable verbose logging (use -v for info, -vv for debug)")

	// Search flags
	root.Flags().StringVarP(&searchCmd.Query, "query", "q", "", "Search query")
	root.Flags().BoolVar(&searchCmd.Saved, "saved", false, "Search only saved/bookmarked posts")
	root.Flags().BoolVar(&searchCmd.Liked, "liked", false, "Search only liked posts")
	root.Flags().BoolVarP(&searchCmd.Force, "force", "f", false, "Force re-index before searching")
	root.Flags().IntVar(&searchCmd.Limit, "limit", 0, "Limit for re-index (0 = no limit)")

	root.AddCommand(loginCmd())
	root.AddCommand(whoamiCmd())
	root.AddCommand(refreshCmd())

	return root
}

func (sc *SearchCmd) Run(c *cobra.Command, args []string) error {
	query := sc.Query
	if len(args) > 0 && args[0] != "" {
		query = strings.Join(args, " ")
	}

	if strings.TrimSpace(query) == "" {
		return c.Help()
	}

	if sc.Saved && sc.Liked {
		return fmt.Errorf("cannot use both --saved and --liked flags")
	}

	if sc.Force {
		logger.Info("Force re-index requested")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		client, err := NewBlueskyClient(ctx)
		if err != nil {
			return fmt.Errorf("failed to create client for re-index: %w", err)
		}

		if err := client.RefreshAndIndex(ctx, sc.Limit); err != nil {
			return fmt.Errorf("failed to re-index: %w", err)
		}
		fmt.Println("✓ Re-index complete")
	}

	source := ""
	if sc.Saved {
		source = "saved"
	} else if sc.Liked {
		source = "liked"
	}

	logger.Info("Searching", "query", query, "source", source)
	results, err := SearchPosts(query, source)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		fmt.Println()
		fmt.Println(emptyStyle.Render("No results found."))

		postCount, _ := CountPosts()
		if postCount == 0 {
			fmt.Println()
			fmt.Println("Your database is empty. Run:")
			fmt.Println("  bsky-browser refresh")
			fmt.Println()
		}
		return nil
	}

	sc.displayResults(results)
	return nil
}

func (sc *SearchCmd) displayResults(results []SearchResult) {
	fmt.Println()
	for i, result := range results {
		number := numberStyle.Render(fmt.Sprintf("[%d]", i+1))
		handle := handleStyle.Render("@" + result.AuthorHandle)

		dateStr := result.CreatedAt.Format("2006-01-02")
		date := dateStyle.Render(dateStr)

		likeCount := likeStyle.Render(fmt.Sprintf("♥ %d", result.LikeCount))

		text := result.Text
		if len(text) > 200 {
			text = text[:200] + "..."
		}

		meta := handle + metaSeparator() + date + metaSeparator() + likeCount

		url := sc.buildPostURL(result.URI, result.AuthorHandle)

		fmt.Printf("%s %s\n", number, meta)
		fmt.Printf("    %s\n", textStyle.Render(text))
		fmt.Printf("    %s\n", urlStyle.Render(url))
		fmt.Println()
	}

	fmt.Println(summaryStyle.Render(fmt.Sprintf("Showing %d results", len(results))))
}

// buildPostURL converts an AT URI to a bsky.app URL
// at://did:plc:.../app.bsky.feed.post/rkey
// -> https://bsky.app/profile/handle/post/rkey
func (sc *SearchCmd) buildPostURL(uri, handle string) string {
	parts := strings.Split(uri, "/")
	if len(parts) >= 2 {
		rkey := parts[len(parts)-1]
		return fmt.Sprintf("https://bsky.app/profile/%s/post/%s", handle, rkey)
	}
	return uri
}

func loginCmd() *cobra.Command {
	var handle string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to Bluesky via OAuth",
		Long:  "Opens a browser to authenticate with Bluesky using OAuth. After successful authentication, your tokens will be stored locally.",
		Example: `  # Interactive login (prompts for handle)
  bsky-browser login

  # Non-interactive login
  bsky-browser login --handle yourname.bsky.social`,
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

func whoamiCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Display current user information",
		Long:  "Shows the handle and DID of the currently authenticated user. The handle is resolved from the DID via an API call and cached. Use --force to refresh the cached handle.",
		Example: `  # Show current user
  bsky-browser whoami

  # Force refresh handle from API
  bsky-browser whoami -f`,
		RunE: func(cmd *cobra.Command, args []string) error {
			am := NewAuthManager()
			auth, err := am.Whoami(force)
			if err != nil {
				return err
			}

			fmt.Printf("%s %s\n", keyStyle.Render("Username:"), auth.Handle)
			fmt.Printf("%s %s\n", keyStyle.Render("DID:"), auth.DID)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force refresh of handle resolution")

	return cmd
}

func refreshCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:     "refresh",
		Aliases: []string{"index"},
		Short:   "Fetch and index all bookmarks and likes",
		Long:    "Fetches all your saved bookmarks and liked posts from Bluesky and indexes them into the local SQLite database for offline searching.",
		Example: `  # Fetch all bookmarks and likes
  bsky-browser refresh

  # Fetch only 50 posts for testing
  bsky-browser refresh --limit 50`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
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
