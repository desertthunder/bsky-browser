package main

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/atproto/auth/oauth"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

// BlueskyClient wraps an authenticated OAuth session
type BlueskyClient struct {
	session *oauth.ClientSession
	auth    *Auth
}

// NewBlueskyClient creates a new authenticated client from stored session
func NewBlueskyClient(ctx context.Context) (*BlueskyClient, error) {
	auth, err := GetAuth()
	if err != nil {
		return nil, fmt.Errorf("failed to load auth: %w", err)
	}
	if auth == nil {
		return nil, fmt.Errorf("not authenticated, please run 'login' first")
	}

	if auth.SessionID == "" {
		return nil, fmt.Errorf("session not found, please run 'login' again")
	}

	did, err := syntax.ParseDID(auth.DID)
	if err != nil {
		return nil, fmt.Errorf("invalid DID in database: %w", err)
	}

	redirectURI := "http://127.0.0.1/callback"
	scopes := []string{"atproto", "transition:generic"}
	config := oauth.NewLocalhostConfig(redirectURI, scopes)

	store := oauth.NewMemStore()

	sessionData := oauth.ClientSessionData{
		AccountDID:                   did,
		SessionID:                    auth.SessionID,
		HostURL:                      auth.PDSURL,
		AuthServerURL:                auth.AuthServerURL,
		AuthServerTokenEndpoint:      auth.AuthServerTokenEndpoint,
		AuthServerRevocationEndpoint: auth.AuthServerRevocationEndpoint,
		AccessToken:                  auth.AccessJWT,
		RefreshToken:                 auth.RefreshJWT,
		Scopes:                       scopes,
		DPoPAuthServerNonce:          auth.DPoPAuthNonce,
		DPoPHostNonce:                auth.DPoPHostNonce,
		DPoPPrivateKeyMultibase:      auth.DPoPPrivateKey,
	}

	if err := store.SaveSession(ctx, sessionData); err != nil {
		return nil, fmt.Errorf("failed to save session to store: %w", err)
	}

	app := oauth.NewClientApp(&config, store)

	session, err := app.ResumeSession(ctx, did, auth.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to resume session: %w", err)
	}

	// TODO: OAuth token refresh fails with "invalid_grant: Token was not issued to this client"
	// This appears to be an issue with how the OAuth client ID is configured or how the indigo
	// library handles token refresh. The current workaround is to fall back to the existing
	// access token, which works until it expires. A proper fix would require investigating
	// the OAuth client configuration and ensuring the client_id matches what the auth server expects.
	newAccessToken, err := session.RefreshTokens(ctx)
	if err != nil {
		logger.Warn("failed to refresh tokens, trying to use existing", "error", err)
	} else if newAccessToken != "" {
		logger.Debug("tokens refreshed successfully")
		auth.AccessJWT = newAccessToken
		if err := UpsertAuth(auth); err != nil {
			logger.Warn("failed to save refreshed tokens", "error", err)
		}
	}

	return &BlueskyClient{
		session: session,
		auth:    auth,
	}, nil
}

// PostResult carries either a Post or an error from fetching
type PostResult struct {
	Post  *Post
	Error error
}

// fetchBookmarks writes bookmarks to the provided channel in batches
func (c *BlueskyClient) fetchBookmarks(ctx context.Context, maxPosts int, ch chan<- *PostResult) {
	logger.Info("Fetching bookmarks", "max", maxPosts)

	apiClient := c.session.APIClient()
	var cursor string
	batchSize := int64(100)
	count := 0

	for {
		resp, err := bsky.BookmarkGetBookmarks(ctx, apiClient, cursor, batchSize)
		if err != nil {
			ch <- &PostResult{Error: fmt.Errorf("failed to fetch bookmarks: %w", err)}
			return
		}

		for _, bookmark := range resp.Bookmarks {
			if bookmark.Item == nil {
				continue
			}

			if bookmark.Item.FeedDefs_PostView != nil {
				pv := bookmark.Item.FeedDefs_PostView

				exists, err := PostExists(pv.Uri)
				if err != nil {
					logger.Warn("failed to check if post exists", "uri", pv.Uri, "error", err)
					continue
				}
				if exists {
					logger.Debug("skipping already indexed post", "uri", pv.Uri)
					continue
				}

				post := c.convertPostView(pv, "saved")
				if post != nil {
					ch <- &PostResult{Post: post}
					count++

					if maxPosts > 0 && count >= maxPosts {
						logger.Info("Reached bookmark limit", "limit", maxPosts)
						return
					}
				}
			}
		}

		logger.Debug("Fetched bookmarks batch", "count", len(resp.Bookmarks), "total", count)

		if resp.Cursor == nil || *resp.Cursor == "" {
			break
		}
		cursor = *resp.Cursor
	}

	logger.Info("Finished fetching bookmarks", "total", count)
}

// fetchLikes writes likes to the provided channel in batches
func (c *BlueskyClient) fetchLikes(ctx context.Context, maxPosts int, ch chan<- *PostResult) {
	logger.Info("Fetching likes", "actor", c.auth.DID, "max", maxPosts)

	apiClient := c.session.APIClient()
	var cursor string
	batchSize := int64(100)
	count := 0

	for {
		resp, err := bsky.FeedGetActorLikes(ctx, apiClient, c.auth.DID, cursor, batchSize)
		if err != nil {
			ch <- &PostResult{Error: fmt.Errorf("failed to fetch likes: %w", err)}
			return
		}

		for _, feedView := range resp.Feed {
			if feedView.Post != nil {
				pv := feedView.Post

				exists, err := PostExists(pv.Uri)
				if err != nil {
					logger.Warn("failed to check if post exists", "uri", pv.Uri, "error", err)
					continue
				}
				if exists {
					logger.Debug("skipping already indexed post", "uri", pv.Uri)
					continue
				}

				post := c.convertPostView(pv, "liked")
				if post != nil {
					ch <- &PostResult{Post: post}
					count++

					if maxPosts > 0 && count >= maxPosts {
						logger.Info("Reached likes limit", "limit", maxPosts)
						return
					}
				}
			}
		}

		logger.Debug("Fetched likes batch", "count", len(resp.Feed), "total", count)

		if resp.Cursor == nil || *resp.Cursor == "" {
			break
		}
		cursor = *resp.Cursor
	}

	logger.Info("Finished fetching likes", "total", count)
}

// batchWriter reads from channel and inserts posts in batches of 10
func batchWriter(ctx context.Context, ch <-chan *PostResult, batchSize int) (int, int, error) {
	batch := make([]*Post, 0, batchSize)
	successCount := 0
	errorCount := 0

	flushBatch := func() error {
		if len(batch) == 0 {
			return nil
		}

		logger.Debug("Inserting batch", "size", len(batch))
		for _, post := range batch {
			if err := InsertPost(post); err != nil {
				logger.Warn("failed to insert post", "uri", post.URI, "error", err)
				errorCount++
			} else {
				successCount++
			}
		}
		batch = batch[:0]
		return nil
	}

	for {
		select {
		case result, ok := <-ch:
			if !ok {
				if err := flushBatch(); err != nil {
					return successCount, errorCount, err
				}
				return successCount, errorCount, nil
			}

			if result.Error != nil {
				return successCount, errorCount, result.Error
			}

			batch = append(batch, result.Post)

			if len(batch) >= batchSize {
				if err := flushBatch(); err != nil {
					return successCount, errorCount, err
				}
			}

		case <-ctx.Done():
			if err := flushBatch(); err != nil {
				return successCount, errorCount, err
			}
			return successCount, errorCount, ctx.Err()
		}
	}
}

// RefreshAndIndex fetches all bookmarks and likes concurrently with batch writes
//
// Creates a channel for bookmarks and likes, starts two goroutines to fetch
// them  concurrently, waits for them to complete, and then closes the channel.
// It then starts a batch writer that reads from the channel and inserts posts
// into the database in batches of 10.
func (c *BlueskyClient) RefreshAndIndex(ctx context.Context, limit int) error {
	logger.Info("Starting refresh and index", "limit", limit)

	postCh := make(chan *PostResult, 100)
	batchSize := 10

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		c.fetchBookmarks(ctx, limit, postCh)
	}()

	go func() {
		defer wg.Done()
		c.fetchLikes(ctx, limit, postCh)
	}()

	go func() {
		wg.Wait()
		close(postCh)
	}()

	successCount, errorCount, err := batchWriter(ctx, postCh, batchSize)
	if err != nil {
		return fmt.Errorf("batch writer error: %w", err)
	}

	logger.Info("Refresh and index complete",
		"success", successCount,
		"errors", errorCount,
		"total", successCount+errorCount)

	if errorCount > 0 {
		logger.Warn("Some posts failed to insert", "error_count", errorCount)
	}

	return nil
}

// convertPostView converts a FeedDefs_PostView to our Post struct
func (c *BlueskyClient) convertPostView(pv *bsky.FeedDefs_PostView, source string) *Post {
	if pv == nil {
		return nil
	}

	record, err := c.parsePostRecord(pv.Record)
	if err != nil {
		logger.Warn("failed to parse post record", "uri", pv.Uri, "error", err)
		record = &postRecord{Text: "", CreatedAt: pv.IndexedAt}
	}

	var authorDID, authorHandle string
	if pv.Author != nil {
		authorDID = pv.Author.Did
		authorHandle = pv.Author.Handle
	}

	likeCount := 0
	if pv.LikeCount != nil {
		likeCount = int(*pv.LikeCount)
	}

	repostCount := 0
	if pv.RepostCount != nil {
		repostCount = int(*pv.RepostCount)
	}

	replyCount := 0
	if pv.ReplyCount != nil {
		replyCount = int(*pv.ReplyCount)
	}

	createdAt, err := syntax.ParseDatetimeLenient(record.CreatedAt)
	if err != nil {
		createdAt, _ = syntax.ParseDatetimeLenient(pv.IndexedAt)
	}

	return &Post{
		URI:          pv.Uri,
		CID:          pv.Cid,
		AuthorDID:    authorDID,
		AuthorHandle: authorHandle,
		Text:         record.Text,
		CreatedAt:    createdAt.Time(),
		LikeCount:    likeCount,
		RepostCount:  repostCount,
		ReplyCount:   replyCount,
		Source:       source,
	}
}

// postRecord represents the expected structure of a post record
type postRecord struct {
	Text      string `json:"text"`
	CreatedAt string `json:"createdAt"`
}

// parsePostRecord extracts post data from the LexiconTypeDecoder
func (c *BlueskyClient) parsePostRecord(decoder interface{}) (*postRecord, error) {
	if decoder == nil {
		return &postRecord{Text: "", CreatedAt: ""}, nil
	}

	type lexDecoder struct{ Val any }

	d, ok := decoder.(*lexDecoder)
	if !ok {
		switch v := decoder.(type) {
		case *bsky.FeedPost:
			return &postRecord{
				Text:      v.Text,
				CreatedAt: v.CreatedAt,
			}, nil
		case bsky.FeedPost:
			return &postRecord{
				Text:      v.Text,
				CreatedAt: v.CreatedAt,
			}, nil
		default:
			return c.parsePostRecordWithReflection(decoder)
		}
	}

	if d.Val == nil {
		return &postRecord{Text: "", CreatedAt: ""}, nil
	}

	if feedPost, ok := d.Val.(*bsky.FeedPost); ok {
		return &postRecord{
			Text:      feedPost.Text,
			CreatedAt: feedPost.CreatedAt,
		}, nil
	}

	return &postRecord{Text: "", CreatedAt: ""}, fmt.Errorf("unknown record type in decoder: %T", d.Val)
}

// parsePostRecordWithReflection uses reflection to access the Val field
func (c *BlueskyClient) parsePostRecordWithReflection(decoder any) (*postRecord, error) {
	val := reflect.ValueOf(decoder)
	if val.Kind() == reflect.Pointer {
		val = val.Elem()
	}

	valField := val.FieldByName("Val")
	if !valField.IsValid() {
		return &postRecord{Text: "", CreatedAt: ""}, fmt.Errorf("no Val field found in %T", decoder)
	}

	actualVal := valField.Interface()
	if actualVal == nil {
		return &postRecord{Text: "", CreatedAt: ""}, nil
	}

	if feedPost, ok := actualVal.(*bsky.FeedPost); ok {
		return &postRecord{
			Text:      feedPost.Text,
			CreatedAt: feedPost.CreatedAt,
		}, nil
	}

	return &postRecord{Text: "", CreatedAt: ""}, fmt.Errorf("unknown record type in Val: %T", actualVal)
}
