# TODO

## Milestone 1 — Scaffold & Logging

- [x] Set up Go module and initial dependencies (`cobra`, `fang`, `lipgloss`)
- [x] Implement `logging.go` — `NewLogger` with `io.MultiWriter(file, stderr)`
- [x] Implement `main.go` — `fang.Execute(ctx, rootCmd)`
- [x] Implement `cli.go` — root command skeleton + `--verbose` flag wiring

## Milestone 2 — Database

- [x] Add `mattn/go-sqlite3` dependency
- [x] Implement `database.go` — `Open()`, auto-migrate schema on startup
- [x] Create `posts` table (uri PK, cid, author, text, timestamps, source)
- [x] Create `auth` table (did PK, handle, tokens, pds_url)
- [x] Create `posts_fts` FTS5 virtual table + sync triggers (insert/update/delete)
- [x] Write `InsertPost`, `UpsertAuth`, `SearchPosts` methods

## Milestone 3 — Authentication

- [ ] Add `bluesky-social/indigo` dependency
- [ ] Implement `auth.go` — loopback OAuth client via `oauth.NewLocalhostConfig`
- [ ] Handle PKCE + DPoP (provided by indigo SDK)
- [ ] Persist tokens to SQLite via `database.go`
- [ ] Implement token refresh on expired access token
- [ ] Wire `login` subcommand in `cli.go` — opens browser, waits for callback
- [ ] Wire `whoami` subcommand — load session from DB, print handle + DID

## Milestone 4 — Fetching Posts

- [ ] Implement `client.go` — authenticated XRPC client from stored session
- [ ] Implement `FetchBookmarks` — paginate `app.bsky.bookmark.getBookmarks`
- [ ] Implement `FetchLikes` — paginate `app.bsky.feed.getActorLikes`
- [ ] Map API responses to `posts` table rows (normalize text, author, timestamps)
- [ ] Wire `refresh` / `index` subcommand — fetch all + insert into DB

## Milestone 5 — Search & Output

- [ ] Implement FTS5 search query with BM25 ranking in `database.go`
- [ ] Support `--saved` / `--liked` source filtering
- [ ] Wire root command to accept positional arg or `-q` flag as search query
- [ ] Wire `-f` flag to force re-index before searching
- [ ] Styled search result output with `lipgloss` (handle, date, like count, text, URL)
- [ ] Handle empty results and no-index-yet states gracefully

## Milestone 6 — Polish

- [ ] Styled `whoami` output with `lipgloss`
- [ ] Log file rotation or max size guard
- [ ] Graceful error messages for network failures, auth expiry, missing DB
- [ ] `--help` examples on each subcommand
- [ ] README with install and usage instructions
