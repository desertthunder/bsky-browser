# BlueSky Browser

A Golang CLI (`bsky-browser`) that uses **lipgloss/fang** (wrapper around cobra) to search, browse, and manage a user's Bluesky saved/bookmarked and liked posts offline via SQLite FTS5.

## Build

```bash
go build -o ./tmp/bsky-browser .
```

## Usage

```bash
./tmp/bsky-browser login          # Opens browser for AT Protocol OAuth
./tmp/bsky-browser whoami         # Prints authenticated user info
./tmp/bsky-browser refresh        # Fetches all saved+liked posts & indexes them
./tmp/bsky-browser index          # Alias for refresh
./tmp/bsky-browser "query"        # Searches all indexed posts
```

Run with `--help` for more information.
