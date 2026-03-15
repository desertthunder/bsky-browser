package main

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
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
	UpdatedAt  time.Time
}

type SearchResult struct {
	Post
	Rank float64
}

func Open(dbPath string) error {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	var err error
	db, err = sql.Open("sqlite3", dbPath+"?_pragma=foreign_keys(1)")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	if err := runMigrations(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

func runMigrations() error {
	schema, err := migrationsFS.ReadFile("migrations/000_initial_schema.sql")
	if err != nil {
		return fmt.Errorf("failed to read schema: %w", err)
	}

	if _, err := db.Exec(string(schema)); err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	return nil
}

func InsertPost(post *Post) error {
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

	return err
}

func UpsertAuth(auth *Auth) error {
	query := `
		INSERT INTO auth (did, handle, access_jwt, refresh_jwt, pds_url, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(did) DO UPDATE SET
			handle = excluded.handle,
			access_jwt = excluded.access_jwt,
			refresh_jwt = excluded.refresh_jwt,
			pds_url = excluded.pds_url,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := db.Exec(query,
		auth.DID,
		auth.Handle,
		auth.AccessJWT,
		auth.RefreshJWT,
		auth.PDSURL,
	)

	return err
}

func GetAuth() (*Auth, error) {
	query := `SELECT did, handle, access_jwt, refresh_jwt, pds_url, updated_at FROM auth LIMIT 1`

	var auth Auth
	var updatedAt string

	err := db.QueryRow(query).Scan(
		&auth.DID,
		&auth.Handle,
		&auth.AccessJWT,
		&auth.RefreshJWT,
		&auth.PDSURL,
		&updatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	auth.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &auth, nil
}

func SearchPosts(query string, source string) ([]SearchResult, error) {
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

	return results, rows.Err()
}

func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}
