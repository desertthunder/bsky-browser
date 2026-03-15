# BlueSky Browser

A Golang CLI (`bsky-browser`) to search & browse your Bluesky saved/bookmarked and liked posts.

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
