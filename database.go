package main

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var db *sql.DB

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Post struct {
	URI          string
	CID          string
	AuthorDID    string
	AuthorHandle string
	Text         string
	CreatedAt    time.Time
	LikeCount    int
	RepostCount  int
	ReplyCount   int
	Source       string
	IndexedAt    time.Time
}

type Auth struct {
	DID        string
	Handle     string
	AccessJWT  string
	RefreshJWT string
	PDSURL     string
	SessionID  string
	UpdatedAt  time.Time
}

type SearchResult struct {
	Post
	Rank float64
}

func Open(dbPath string) error {
	logger.Debug("opening database", "path", dbPath)

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	var err error
	db, err = sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Debug("database connection established")

	if err := runMigrations(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	logger.Debug("database migrations completed successfully")
	return nil
}

func runMigrations() error {
	migrations := []string{
		"migrations/000_initial_schema.sql",
		"migrations/001_add_session_id.sql",
	}

	for _, migration := range migrations {
		content, err := migrationsFS.ReadFile(migration)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", migration, err)
		}

		if _, err := db.Exec(string(content)); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return fmt.Errorf("failed to execute migration %s: %w", migration, err)
			}
		}
	}

	return nil
}

func InsertPost(post *Post) error {
	logger.Debug("inserting post", "uri", post.URI, "author", post.AuthorHandle)

	query := `
		INSERT INTO posts (uri, cid, author_did, author_handle, text, created_at, like_count, repost_count, reply_count, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(uri) DO UPDATE SET
			cid = excluded.cid,
			author_did = excluded.author_did,
			author_handle = excluded.author_handle,
			text = excluded.text,
			created_at = excluded.created_at,
			like_count = excluded.like_count,
			repost_count = excluded.repost_count,
			reply_count = excluded.reply_count,
			source = excluded.source,
			indexed_at = CURRENT_TIMESTAMP
	`

	_, err := db.Exec(query,
		post.URI,
		post.CID,
		post.AuthorDID,
		post.AuthorHandle,
		post.Text,
		post.CreatedAt,
		post.LikeCount,
		post.RepostCount,
		post.ReplyCount,
		post.Source,
	)

	if err != nil {
		logger.Error("failed to insert post", "uri", post.URI, "error", err)
	}

	return err
}

func UpsertAuth(auth *Auth) error {
	logger.Debug("upserting auth", "did", auth.DID, "handle", auth.Handle)

	query := `
		INSERT INTO auth (did, handle, access_jwt, refresh_jwt, pds_url, session_id, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(did) DO UPDATE SET
			handle = excluded.handle,
			access_jwt = excluded.access_jwt,
			refresh_jwt = excluded.refresh_jwt,
			pds_url = excluded.pds_url,
			session_id = excluded.session_id,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := db.Exec(query,
		auth.DID,
		auth.Handle,
		auth.AccessJWT,
		auth.RefreshJWT,
		auth.PDSURL,
		auth.SessionID,
	)

	if err != nil {
		logger.Error("failed to upsert auth", "did", auth.DID, "error", err)
	}

	return err
}

func GetAuth() (*Auth, error) {
	logger.Debug("loading auth from database")

	query := `SELECT did, handle, access_jwt, refresh_jwt, pds_url, session_id, updated_at FROM auth LIMIT 1`

	var auth Auth
	var updatedAt string

	var sessionID sql.NullString

	err := db.QueryRow(query).Scan(
		&auth.DID,
		&auth.Handle,
		&auth.AccessJWT,
		&auth.RefreshJWT,
		&auth.PDSURL,
		&sessionID,
		&updatedAt,
	)

	if sessionID.Valid {
		auth.SessionID = sessionID.String
	}

	if err == sql.ErrNoRows {
		logger.Debug("no auth record found in database")
		return nil, nil
	}
	if err != nil {
		logger.Error("failed to load auth", "error", err)
		return nil, err
	}

	auth.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	logger.Debug("auth loaded successfully", "did", auth.DID, "handle", auth.Handle)
	return &auth, nil
}

func SearchPosts(query string, source string) ([]SearchResult, error) {
	logger.Debug("searching posts", "query", query, "source", source)

	sql := `
		SELECT p.uri, p.cid, p.author_did, p.author_handle, p.text, p.created_at,
			   p.like_count, p.repost_count, p.reply_count, p.source, p.indexed_at,
			   bm25(posts_fts, 5.0, 1.0) AS rank
		FROM posts_fts
		JOIN posts p ON posts_fts.rowid = p.rowid
		WHERE posts_fts MATCH ?
		  AND (? = '' OR p.source = ?)
		ORDER BY rank
		LIMIT 25
	`

	rows, err := db.Query(sql, query, source, source)
	if err != nil {
		logger.Error("failed to execute search query", "error", err)
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var createdAt, indexedAt string

		err := rows.Scan(
			&r.URI,
			&r.CID,
			&r.AuthorDID,
			&r.AuthorHandle,
			&r.Text,
			&createdAt,
			&r.LikeCount,
			&r.RepostCount,
			&r.ReplyCount,
			&r.Source,
			&indexedAt,
			&r.Rank,
		)
		if err != nil {
			return nil, err
		}

		r.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		r.IndexedAt, _ = time.Parse("2006-01-02 15:04:05", indexedAt)
		results = append(results, r)
	}

	logger.Debug("search completed", "results", len(results))
	return results, rows.Err()
}

func Close() error {
	logger.Debug("closing database connection")
	if db != nil {
		err := db.Close()
		if err != nil {
			logger.Error("failed to close database", "error", err)
			return err
		}
		logger.Debug("database connection closed")
	}
	return nil
}
