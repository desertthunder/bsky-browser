<!-- markdownlint-disable MD033 -->
# BlueSky Browser

A Golang CLI (`bsky-browser`) that lets you search, browse, and manage your Bluesky saved/bookmarked and liked posts offline via SQLite FTS5.

## Features

- **OAuth Authentication** - Secure AT Protocol OAuth login with automatic token refresh
- **Offline Indexing** - Download all your bookmarks and likes to a local SQLite database
- **Full-Text Search** - Fast FTS5-powered search with BM25 ranking
- **Source Filtering** - Search all posts, saved only, or liked only
- **Styled Output** - Beautiful terminal UI using Charm's Lipgloss

## Build

```bash
go build -o ./tmp/bsky-browser .
```

## Usage

<details>
<summary>
Authentication
</summary>

```bash
# Login to Bluesky (opens browser for OAuth)
./tmp/bsky-browser login
./tmp/bsky-browser login --handle yourhandle.bsky.social  # Non-interactive

# Check who you're logged in as
./tmp/bsky-browser whoami
./tmp/bsky-browser whoami -f  # Force refresh cached handle from API
```

</details>

<details>
<summary>
Indexing
</summary>

```bash
# Fetch all bookmarks and likes (can take a while)
./tmp/bsky-browser refresh
./tmp/bsky-browser index      # Alias for refresh

# Limit for testing (fetches 10 bookmarks + 10 likes = 20 total)
./tmp/bsky-browser refresh --limit 10
```

</details>

<details>
<summary>
Search
</summary>

```bash
# Search all indexed posts
./tmp/bsky-browser "search query"
./tmp/bsky-browser -q "search query"  # Explicit flag

# Search only bookmarks
./tmp/bsky-browser "query" --saved

# Search only likes
./tmp/bsky-browser "query" --liked

# Force re-index before searching (with optional limit)
./tmp/bsky-browser "query" -f
./tmp/bsky-browser "query" -f --limit 20
```

</details>

<details>
<summary>
Logging
</summary>

```bash
# Default: logs written to file only (no stderr output)
./tmp/bsky-browser "query"

# -v: Info level logs to stderr
./tmp/bsky-browser -v "query"

# -vv: Debug level logs to stderr
./tmp/bsky-browser -vv "query"
./tmp/bsky-browser -vv whoami
```

</details>

## Data Storage

- **Database**: `~/.config/bsky-browser/bsky-browser.db`
- **Logs**: `~/.config/bsky-browser/logs/bsky-browser_*.log` (timestamped)

## Requirements

- Go 1.24+
- SQLite (via modernc.org/sqlite)

## License

MIT
