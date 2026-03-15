package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/auth/oauth"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

type AuthManager struct {
	app      *oauth.ClientApp
	server   *http.Server
	listener net.Listener
	codeChan chan string
	errChan  chan error
	port     int
}

func NewAuthManager() *AuthManager {
	return &AuthManager{
		codeChan: make(chan string, 1),
		errChan:  make(chan error, 1),
	}
}

func (am *AuthManager) Login(ctx context.Context, handle string) error {
	logger.Info("Starting OAuth login flow", "handle", handle)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}
	am.listener = listener
	am.port = listener.Addr().(*net.TCPAddr).Port
	logger.Debugf("Started local listener on port %d", am.port)

	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", am.port)
	scopes := []string{"atproto", "transition:generic"}

	config := oauth.NewLocalhostConfig(redirectURI, scopes)
	store := oauth.NewMemStore()
	am.app = oauth.NewClientApp(&config, store)

	logger.Debug("Starting auth flow", "identifier", handle)
	redirectURL, err := am.app.StartAuthFlow(ctx, handle)
	if err != nil {
		return fmt.Errorf("failed to start auth flow: %w", err)
	}
	logger.Info("Authorization URL generated", "url", redirectURL)

	am.startCallbackServer()

	logger.Info("Opening browser for authentication...")
	if err := openBrowser(redirectURL); err != nil {
		logger.Warn("Failed to open browser, please open manually", "url", redirectURL)
		fmt.Printf("Please open this URL in your browser:\n%s\n", redirectURL)
	}

	select {
	case code := <-am.codeChan:
		logger.Info("Received authorization code")
		return am.exchangeCode(ctx, code)
	case err := <-am.errChan:
		return fmt.Errorf("authorization error: %w", err)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (am *AuthManager) startCallbackServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		code := query.Get("code")
		if code == "" {
			errMsg := query.Get("error")
			if errMsg == "" {
				errMsg = "missing authorization code"
			}
			errDesc := query.Get("error_description")
			am.errChan <- fmt.Errorf("authorization failed: %s - %s", errMsg, errDesc)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Authorization failed: %s\n", errMsg)
			return
		}

		state := query.Get("state")
		iss := query.Get("iss")
		logger.Debug("callback received", "code", code[:8]+"...", "state", state, "iss", iss)
		am.codeChan <- fmt.Sprintf("%s|%s|%s", code, state, iss)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Authorization successful! You can close this window.")
	})

	am.server = &http.Server{
		Handler: mux,
	}

	go func() {
		if err := am.server.Serve(am.listener); err != nil && err != http.ErrServerClosed {
			am.errChan <- err
		}
	}()
}

func (am *AuthManager) exchangeCode(ctx context.Context, data string) error {
	logger.Debug("Processing callback and exchanging code for tokens")

	parts := strings.SplitN(data, "|", 3)
	if len(parts) < 2 {
		return fmt.Errorf("invalid callback data")
	}

	params := make(map[string][]string)
	params["code"] = []string{parts[0]}
	params["state"] = []string{parts[1]}
	if len(parts) > 2 && parts[2] != "" {
		params["iss"] = []string{parts[2]}
	}

	sessData, err := am.app.ProcessCallback(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to process callback: %w", err)
	}

	logger.Info("Successfully authenticated",
		"did", sessData.AccountDID,
		"session_id", sessData.SessionID,
		"pds", sessData.HostURL,
		"auth_server", sessData.AuthServerURL,
	)

	auth := &Auth{
		DID:                          sessData.AccountDID.String(),
		Handle:                       sessData.AccountDID.String(),
		AccessJWT:                    sessData.AccessToken,
		RefreshJWT:                   sessData.RefreshToken,
		PDSURL:                       sessData.HostURL,
		SessionID:                    sessData.SessionID,
		AuthServerURL:                sessData.AuthServerURL,
		AuthServerTokenEndpoint:      sessData.AuthServerTokenEndpoint,
		AuthServerRevocationEndpoint: sessData.AuthServerRevocationEndpoint,
		DPoPAuthNonce:                sessData.DPoPAuthServerNonce,
		DPoPHostNonce:                sessData.DPoPHostNonce,
		DPoPPrivateKey:               sessData.DPoPPrivateKeyMultibase,
		UpdatedAt:                    time.Now(),
	}

	if err := UpsertAuth(auth); err != nil {
		return fmt.Errorf("failed to persist auth: %w", err)
	}

	logger.Info("Authentication saved to database")
	return nil
}

func (am *AuthManager) RefreshSession(ctx context.Context) (*Auth, error) {
	auth, err := GetAuth()
	if err != nil {
		return nil, fmt.Errorf("failed to load auth: %w", err)
	}
	if auth == nil {
		return nil, fmt.Errorf("no session found, please run 'login' first")
	}

	if auth.SessionID == "" {
		logger.Debug("no session_id stored, cannot refresh tokens")
		return auth, nil
	}

	logger.Debug("resuming session to check/refresh tokens", "did", auth.DID, "session_id", auth.SessionID)

	// TODO: persist the OAuth store
	redirectURI := "http://127.0.0.1/callback"
	scopes := []string{"atproto", "transition:generic"}
	config := oauth.NewLocalhostConfig(redirectURI, scopes)
	store := oauth.NewMemStore()
	app := oauth.NewClientApp(&config, store)

	did, err := syntax.ParseDID(auth.DID)
	if err != nil {
		return nil, fmt.Errorf("invalid DID in database: %w", err)
	}

	session, err := app.ResumeSession(ctx, did, auth.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to resume session: %w", err)
	}

	newAccessToken, err := session.RefreshTokens(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh tokens: %w", err)
	}

	if newAccessToken != "" {
		logger.Info("tokens refreshed successfully")
		auth.AccessJWT = newAccessToken
		auth.UpdatedAt = time.Now()
		if err := UpsertAuth(auth); err != nil {
			logger.Warn("failed to update refreshed tokens in database", "error", err)
		}
	} else {
		logger.Debug("tokens still valid, no refresh needed")
	}

	return auth, nil
}

func (am *AuthManager) Whoami(force bool) (*Auth, error) {
	auth, err := GetAuth()
	if err != nil {
		return nil, fmt.Errorf("failed to load auth: %w", err)
	}
	if auth == nil {
		return nil, fmt.Errorf("not logged in")
	}

	if force || strings.HasPrefix(auth.Handle, "did:") {
		logger.Debugf("resolving DID %s to handle", auth.DID)

		did, err := syntax.ParseDID(auth.DID)
		if err != nil {
			return nil, fmt.Errorf("invalid DID in database: %w", err)
		}

		dir := &identity.BaseDirectory{}
		ident, err := dir.LookupDID(context.Background(), did)
		if err != nil {
			logger.Warn("failed to resolve handle, using DID", "error", err)
			return auth, nil
		}

		auth.Handle = ident.Handle.String()

		if err := UpsertAuth(auth); err != nil {
			logger.Warn("failed to cache resolved handle", "error", err)
		}

		logger.Debugf("resolved handle %s", auth.Handle)
	}

	return auth, nil
}

func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}

	return exec.Command(cmd, args...).Start()
}

func getDataDir() string {
	if dir := os.Getenv("BSKY_BROWSER_DATA"); dir != "" {
		return dir
	}

	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, _ := os.UserHomeDir()
		configDir = home + "/.config"
	}
	return filepath.Join(configDir, "bsky-browser")
}
