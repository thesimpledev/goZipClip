package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// youtubeScope only allows uploading videos, nothing else on the
// account.
const youtubeScope = "https://www.googleapis.com/auth/youtube.upload"

// YouTubeClient uploads finished videos with the YouTube Data API,
// using an OAuth client the user created in their own Google Cloud
// project. The refresh token lives in youtube_token.json in the
// per-user config folder.
type YouTubeClient struct {
	clientID  string
	secret    string
	tokenPath string
	authURL   string
	tokenURL  string
	uploadURL string
	client    *http.Client
}

// youtubeToken is the persisted OAuth state.
type youtubeToken struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	Expiry       time.Time `json:"expiry"`
}

// NewYouTubeClient builds a client from the configured credentials.
func NewYouTubeClient(cfg Config) *YouTubeClient {
	// #nosec G101 -- these are Google's fixed public endpoint URLs; the credentials come from the user's config
	return &YouTubeClient{
		clientID:  cfg.YouTubeClientID,
		secret:    cfg.YouTubeClientSecret,
		tokenPath: filepath.Join(configDir(), "youtube_token.json"),
		authURL:   "https://accounts.google.com/o/oauth2/v2/auth",
		tokenURL:  "https://oauth2.googleapis.com/token",
		uploadURL: "https://www.googleapis.com/upload/youtube/v3/videos",
		// Uploads run for a long time on purpose; cancellation comes
		// from the request context, not a client-wide timeout.
		client: &http.Client{},
	}
}

// Authorize runs the OAuth flow for a desktop app: it listens on a
// loopback port, sends the user's browser to Google's consent page
// through openURL, and trades the returned code for tokens.
func (y *YouTubeClient) Authorize(ctx context.Context, openURL func(string) error) error {
	if y == nil || openURL == nil {
		return errors.New("youtube client or browser opener missing")
	}
	if y.clientID == "" || y.secret == "" {
		return errors.New("set the YouTube client ID and secret first")
	}
	listener, listenErr := net.Listen("tcp", "127.0.0.1:0")
	if listenErr != nil {
		return listenErr
	}
	redirect := "http://" + listener.Addr().String()
	state, stateErr := randomState()
	if stateErr != nil {
		return stateErr
	}
	if openErr := openURL(y.consentURL(redirect, state)); openErr != nil {
		// The listener has no accepted connections yet; nothing to drain.
		_ = listener.Close()
		return openErr
	}
	code, codeErr := waitForCode(ctx, listener, state)
	if codeErr != nil {
		return codeErr
	}
	return y.exchangeCode(ctx, code, redirect)
}

func (y *YouTubeClient) consentURL(redirect, state string) string {
	values := url.Values{
		"client_id":     {y.clientID},
		"redirect_uri":  {redirect},
		"response_type": {"code"},
		"scope":         {youtubeScope},
		"access_type":   {"offline"},
		"prompt":        {"consent"},
		"state":         {state},
	}
	return y.authURL + "?" + values.Encode()
}

func randomState() (string, error) {
	buf := make([]byte, 16)
	if _, readErr := rand.Read(buf); readErr != nil {
		return "", readErr
	}
	return hex.EncodeToString(buf), nil
}

// waitForCode serves exactly one OAuth redirect on the listener and
// returns the authorization code, honoring context cancellation.
func waitForCode(ctx context.Context, listener net.Listener, state string) (string, error) {
	if ctx == nil || listener == nil {
		return "", errors.New("missing context or listener")
	}
	type result struct {
		code string
		err  error
	}
	results := make(chan result, 1)
	server := &http.Server{ReadHeaderTimeout: 10 * time.Second}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code, handleErr := codeFromRequest(r, state)
		if handleErr != nil {
			http.Error(w, handleErr.Error(), http.StatusBadRequest)
		} else {
			// The confirmation page write result changes nothing.
			_, _ = fmt.Fprintln(w, "ZipClip is connected to YouTube. You can close this window.")
		}
		select {
		case results <- result{code: code, err: handleErr}:
		default:
		}
	})
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			select {
			case results <- result{err: serveErr}:
			default:
			}
		}
	}()
	defer func() {
		// Shutdown errors after the code arrived change nothing.
		_ = server.Shutdown(context.Background())
	}()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case got := <-results:
		return got.code, got.err
	}
}

func codeFromRequest(r *http.Request, state string) (string, error) {
	query := r.URL.Query()
	if authErr := query.Get("error"); authErr != "" {
		return "", fmt.Errorf("google refused the authorization: %s", authErr)
	}
	if query.Get("state") != state {
		return "", errors.New("authorization response did not match this request")
	}
	code := query.Get("code")
	if code == "" {
		return "", errors.New("authorization response had no code")
	}
	return code, nil
}

func (y *YouTubeClient) exchangeCode(ctx context.Context, code, redirect string) error {
	form := url.Values{
		"client_id":     {y.clientID},
		"client_secret": {y.secret},
		"code":          {code},
		"redirect_uri":  {redirect},
		"grant_type":    {"authorization_code"},
	}
	token, tokenErr := y.tokenRequest(ctx, form)
	if tokenErr != nil {
		return tokenErr
	}
	if token.RefreshToken == "" {
		return errors.New("google returned no refresh token; remove the app's access at myaccount.google.com/permissions and connect again")
	}
	return y.saveToken(token)
}

// tokenRequest posts the form to the token endpoint and parses the
// response into a token, carrying over the old refresh token when the
// response omits one.
func (y *YouTubeClient) tokenRequest(ctx context.Context, form url.Values) (youtubeToken, error) {
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, y.tokenURL, strings.NewReader(form.Encode()))
	if reqErr != nil {
		return youtubeToken{}, reqErr
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	body, doErr := y.do(req, http.StatusOK)
	if doErr != nil {
		return youtubeToken{}, fmt.Errorf("youtube token: %w", doErr)
	}
	var parsed struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if jsonErr := json.Unmarshal(body, &parsed); jsonErr != nil {
		return youtubeToken{}, fmt.Errorf("youtube token response: %w", jsonErr)
	}
	if parsed.AccessToken == "" {
		return youtubeToken{}, errors.New("youtube token response had no access token")
	}
	return youtubeToken{
		AccessToken:  parsed.AccessToken,
		RefreshToken: parsed.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(parsed.ExpiresIn-60) * time.Second),
	}, nil
}

func (y *YouTubeClient) saveToken(token youtubeToken) error {
	// #nosec G117 -- persisting the OAuth token to a 0600 file in the user config folder is the feature; without it every upload would need a fresh browser sign-in
	data, marshalErr := json.MarshalIndent(token, "", "  ")
	if marshalErr != nil {
		return marshalErr
	}
	return os.WriteFile(y.tokenPath, append(data, '\n'), 0o600)
}

func (y *YouTubeClient) loadToken() (youtubeToken, error) {
	// #nosec G304 -- the token path is built from the per-user config folder
	data, readErr := os.ReadFile(y.tokenPath)
	if readErr != nil {
		return youtubeToken{}, errors.New("youtube is not connected yet; press Connect YouTube in Settings")
	}
	var token youtubeToken
	if jsonErr := json.Unmarshal(data, &token); jsonErr != nil {
		return youtubeToken{}, fmt.Errorf("youtube token file: %w", jsonErr)
	}
	return token, nil
}

// freshAccessToken returns a usable access token, refreshing and
// persisting it when the stored one has expired.
func (y *YouTubeClient) freshAccessToken(ctx context.Context) (string, error) {
	token, loadErr := y.loadToken()
	if loadErr != nil {
		return "", loadErr
	}
	if time.Now().Before(token.Expiry) && token.AccessToken != "" {
		return token.AccessToken, nil
	}
	form := url.Values{
		"client_id":     {y.clientID},
		"client_secret": {y.secret},
		"refresh_token": {token.RefreshToken},
		"grant_type":    {"refresh_token"},
	}
	refreshed, refreshErr := y.tokenRequest(ctx, form)
	if refreshErr != nil {
		return "", refreshErr
	}
	if refreshed.RefreshToken == "" {
		refreshed.RefreshToken = token.RefreshToken
	}
	if saveErr := y.saveToken(refreshed); saveErr != nil {
		return "", saveErr
	}
	return refreshed.AccessToken, nil
}

// Upload sends the video to YouTube as a private upload using the
// resumable upload protocol: one request to open the session, one to
// stream the file.
func (y *YouTubeClient) Upload(ctx context.Context, path string) error {
	if y == nil || ctx == nil || path == "" {
		return errors.New("youtube client, context, or file path missing")
	}
	info, statErr := os.Stat(path)
	if statErr != nil {
		return statErr
	}
	access, tokenErr := y.freshAccessToken(ctx)
	if tokenErr != nil {
		return tokenErr
	}
	session, sessionErr := y.openUploadSession(ctx, access, path, info.Size())
	if sessionErr != nil {
		return sessionErr
	}
	return y.sendFile(ctx, access, session, path, info.Size())
}

func (y *YouTubeClient) openUploadSession(ctx context.Context, access, path string, size int64) (string, error) {
	title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	metadata := map[string]any{
		"snippet": map[string]any{"title": title},
		"status":  map[string]any{"privacyStatus": "private"},
	}
	body, marshalErr := json.Marshal(metadata)
	if marshalErr != nil {
		return "", marshalErr
	}
	endpoint := y.uploadURL + "?uploadType=resumable&part=snippet,status"
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if reqErr != nil {
		return "", reqErr
	}
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("X-Upload-Content-Type", "video/mp4")
	req.Header.Set("X-Upload-Content-Length", strconv.FormatInt(size, 10))
	resp, doErr := y.client.Do(req)
	if doErr != nil {
		return "", fmt.Errorf("youtube upload session: %w", doErr)
	}
	defer drainAndClose(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("youtube upload session: status %d: %s", resp.StatusCode, readSnippet(resp.Body))
	}
	session := resp.Header.Get("Location")
	if session == "" {
		return "", errors.New("youtube upload session had no location")
	}
	return session, nil
}

func (y *YouTubeClient) sendFile(ctx context.Context, access, session, path string, size int64) error {
	// #nosec G304 -- the upload path is a finished file in the user's own output folder
	file, openErr := os.Open(path)
	if openErr != nil {
		return openErr
	}
	defer func() {
		// The file was only read; a close error changes nothing.
		_ = file.Close()
	}()
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodPut, session, file)
	if reqErr != nil {
		return reqErr
	}
	req.ContentLength = size
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "video/mp4")
	resp, doErr := y.client.Do(req)
	if doErr != nil {
		return fmt.Errorf("youtube upload: %w", doErr)
	}
	defer drainAndClose(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("youtube upload: status %d: %s", resp.StatusCode, readSnippet(resp.Body))
	}
	return nil
}

// do runs the request, requiring the given status, and returns the
// response body.
func (y *YouTubeClient) do(req *http.Request, wantStatus int) ([]byte, error) {
	// #nosec G704 -- requests only go to the fixed Google endpoints set in NewYouTubeClient; tests override them with local servers
	resp, doErr := y.client.Do(req)
	if doErr != nil {
		return nil, doErr
	}
	defer drainAndClose(resp.Body)
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return nil, readErr
	}
	if resp.StatusCode != wantStatus {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return body, nil
}

func readSnippet(r io.Reader) string {
	body, readErr := io.ReadAll(io.LimitReader(r, 1<<20))
	if readErr != nil {
		return ""
	}
	return truncate(string(body), 200)
}

func drainAndClose(body io.ReadCloser) {
	// Draining lets the connection be reused; errors change nothing.
	_, _ = io.Copy(io.Discard, io.LimitReader(body, 1<<20))
	_ = body.Close()
}
