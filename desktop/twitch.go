package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TwitchClient answers one question: is the channel live right now?
// It uses an app access token obtained with the client credentials
// grant, which only allows public Helix reads.
type TwitchClient struct {
	clientID string
	secret   string
	authURL  string
	apiURL   string
	client   *http.Client

	token    string
	tokenExp time.Time
}

// NewTwitchClient builds a client for the given application
// credentials.
func NewTwitchClient(clientID, secret string) *TwitchClient {
	return &TwitchClient{
		clientID: clientID,
		secret:   secret,
		authURL:  "https://id.twitch.tv/oauth2/token",
		apiURL:   "https://api.twitch.tv/helix",
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *TwitchClient) tokenRequest(ctx context.Context) (*http.Request, error) {
	form := url.Values{
		"client_id":     {t.clientID},
		"client_secret": {t.secret},
		"grant_type":    {"client_credentials"},
	}
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, t.authURL, strings.NewReader(form.Encode()))
	if reqErr != nil {
		return nil, reqErr
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req, nil
}

func (t *TwitchClient) ensureToken(ctx context.Context) error {
	if t.token != "" && time.Now().Before(t.tokenExp) {
		return nil
	}
	req, reqErr := t.tokenRequest(ctx)
	if reqErr != nil {
		return reqErr
	}
	body, doErr := t.do(req)
	if doErr != nil {
		return fmt.Errorf("twitch token: %w", doErr)
	}
	var parsed struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if jsonErr := json.Unmarshal(body, &parsed); jsonErr != nil {
		return fmt.Errorf("twitch token response: %w", jsonErr)
	}
	if parsed.AccessToken == "" {
		return errors.New("twitch token response had no access token")
	}
	t.token = parsed.AccessToken
	t.tokenExp = time.Now().Add(time.Duration(parsed.ExpiresIn-60) * time.Second)
	return nil
}

func (t *TwitchClient) do(req *http.Request) ([]byte, error) {
	// #nosec G704 -- requests only go to the fixed Twitch endpoints set in NewTwitchClient; tests override them with local servers
	resp, doErr := t.client.Do(req)
	if doErr != nil {
		return nil, doErr
	}
	defer func() {
		// The body is fully read below; a close error changes nothing.
		_ = resp.Body.Close()
	}()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return nil, readErr
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return body, nil
}

// IsLive reports whether the channel currently has a live stream.
func (t *TwitchClient) IsLive(ctx context.Context, channel string) (bool, error) {
	if t == nil || channel == "" {
		return false, errors.New("twitch client or channel missing")
	}
	if tokenErr := t.ensureToken(ctx); tokenErr != nil {
		return false, tokenErr
	}
	endpoint := t.apiURL + "/streams?user_login=" + url.QueryEscape(channel)
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if reqErr != nil {
		return false, reqErr
	}
	req.Header.Set("Client-Id", t.clientID)
	req.Header.Set("Authorization", "Bearer "+t.token)
	body, doErr := t.do(req)
	if doErr != nil {
		return false, fmt.Errorf("twitch streams: %w", doErr)
	}
	var parsed struct {
		Data []struct {
			Type string `json:"type"`
		} `json:"data"`
	}
	if jsonErr := json.Unmarshal(body, &parsed); jsonErr != nil {
		return false, fmt.Errorf("twitch streams response: %w", jsonErr)
	}
	return len(parsed.Data) > 0, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
