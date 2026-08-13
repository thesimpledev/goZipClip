package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newTestYouTubeClient points a client at synthesized local servers
// and a temp token file so no real Google endpoint is ever contacted.
func newTestYouTubeClient(t *testing.T, handler http.Handler) *YouTubeClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &YouTubeClient{
		clientID:  "example-client-id",
		secret:    "example-secret",
		tokenPath: filepath.Join(t.TempDir(), "youtube_token.json"),
		authURL:   server.URL + "/auth",
		tokenURL:  server.URL + "/token",
		uploadURL: server.URL + "/upload",
		client:    server.Client(),
	}
}

func writeTestToken(t *testing.T, y *YouTubeClient, token youtubeToken) {
	t.Helper()
	data, marshalErr := json.Marshal(token)
	if marshalErr != nil {
		t.Fatalf("marshal token: %v", marshalErr)
	}
	if writeErr := os.WriteFile(y.tokenPath, data, 0o600); writeErr != nil {
		t.Fatalf("write token: %v", writeErr)
	}
}

func TestUploadHappyPath(t *testing.T) {
	var gotBytes int64
	mux := http.NewServeMux()
	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer example-access" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Location", "http://"+r.Host+"/session")
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/session", func(w http.ResponseWriter, r *http.Request) {
		gotBytes = r.ContentLength
		w.WriteHeader(http.StatusOK)
	})
	client := newTestYouTubeClient(t, mux)
	writeTestToken(t, client, youtubeToken{
		AccessToken:  "example-access",
		RefreshToken: "example-refresh",
		Expiry:       time.Now().Add(time.Hour),
	})
	video := filepath.Join(t.TempDir(), "20260801-v123.mp4")
	if writeErr := os.WriteFile(video, []byte("synthesized video bytes"), 0o600); writeErr != nil {
		t.Fatalf("write video: %v", writeErr)
	}
	if upErr := client.Upload(context.Background(), video); upErr != nil {
		t.Fatalf("upload: %v", upErr)
	}
	if gotBytes != int64(len("synthesized video bytes")) {
		t.Fatalf("uploaded %d bytes", gotBytes)
	}
}

func TestUploadRefreshesExpiredToken(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if parseErr := r.ParseForm(); parseErr != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "example-refresh" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// The test server response writer error is irrelevant.
		_, _ = w.Write([]byte(`{"access_token":"refreshed-access","expires_in":3600}`))
	})
	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer refreshed-access" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Location", "http://"+r.Host+"/session")
	})
	mux.HandleFunc("/session", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	client := newTestYouTubeClient(t, mux)
	writeTestToken(t, client, youtubeToken{
		AccessToken:  "stale-access",
		RefreshToken: "example-refresh",
		Expiry:       time.Now().Add(-time.Hour),
	})
	video := filepath.Join(t.TempDir(), "20260801-v123.mp4")
	if writeErr := os.WriteFile(video, []byte("synthesized"), 0o600); writeErr != nil {
		t.Fatalf("write video: %v", writeErr)
	}
	if upErr := client.Upload(context.Background(), video); upErr != nil {
		t.Fatalf("upload: %v", upErr)
	}
	saved, loadErr := client.loadToken()
	if loadErr != nil {
		t.Fatalf("load token: %v", loadErr)
	}
	if saved.AccessToken != "refreshed-access" || saved.RefreshToken != "example-refresh" {
		t.Fatalf("refreshed token not persisted: %+v", saved)
	}
}

func TestUploadWithoutConnection(t *testing.T) {
	client := newTestYouTubeClient(t, http.NewServeMux())
	video := filepath.Join(t.TempDir(), "20260801-v123.mp4")
	if writeErr := os.WriteFile(video, []byte("synthesized"), 0o600); writeErr != nil {
		t.Fatalf("write video: %v", writeErr)
	}
	upErr := client.Upload(context.Background(), video)
	if upErr == nil || !strings.Contains(upErr.Error(), "Connect YouTube") {
		t.Fatalf("expected a not-connected error, got %v", upErr)
	}
}

func TestCodeFromRequest(t *testing.T) {
	build := func(query string) *http.Request {
		parsed, parseErr := url.Parse("http://127.0.0.1/?" + query)
		if parseErr != nil {
			t.Fatalf("parse: %v", parseErr)
		}
		return &http.Request{URL: parsed}
	}
	if _, err := codeFromRequest(build("state=right&code=example-code"), "right"); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	if _, err := codeFromRequest(build("state=wrong&code=example-code"), "right"); err == nil {
		t.Fatal("state mismatch accepted")
	}
	if _, err := codeFromRequest(build("state=right"), "right"); err == nil {
		t.Fatal("missing code accepted")
	}
	if _, err := codeFromRequest(build("error=access_denied"), "right"); err == nil {
		t.Fatal("refusal accepted")
	}
}
