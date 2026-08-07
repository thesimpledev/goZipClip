package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestTwitchClient points a client at synthesized local servers so
// no real Twitch endpoint is ever contacted from tests.
func newTestTwitchClient(t *testing.T, streamsBody string, streamsStatus int) *TwitchClient {
	t.Helper()
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// The test server response writer error is irrelevant.
		_, _ = w.Write([]byte(`{"access_token":"example-token","expires_in":3600}`))
	}))
	t.Cleanup(auth.Close)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer example-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(streamsStatus)
		// The test server response writer error is irrelevant.
		_, _ = w.Write([]byte(streamsBody))
	}))
	t.Cleanup(api.Close)
	client := NewTwitchClient("example-client-id", "example-secret")
	client.authURL = auth.URL
	client.apiURL = api.URL
	return client
}

func TestIsLiveTrue(t *testing.T) {
	client := newTestTwitchClient(t, `{"data":[{"type":"live"}]}`, http.StatusOK)
	live, liveErr := client.IsLive(context.Background(), "example-channel")
	if liveErr != nil {
		t.Fatalf("unexpected error: %v", liveErr)
	}
	if !live {
		t.Fatal("expected live")
	}
}

func TestIsLiveFalse(t *testing.T) {
	client := newTestTwitchClient(t, `{"data":[]}`, http.StatusOK)
	live, liveErr := client.IsLive(context.Background(), "example-channel")
	if liveErr != nil {
		t.Fatalf("unexpected error: %v", liveErr)
	}
	if live {
		t.Fatal("expected offline")
	}
}

func TestIsLiveServerError(t *testing.T) {
	client := newTestTwitchClient(t, `{"error":"Internal Server Error"}`, http.StatusInternalServerError)
	if _, liveErr := client.IsLive(context.Background(), "example-channel"); liveErr == nil {
		t.Fatal("expected an error for a failing API")
	}
}

func TestIsLiveEmptyChannel(t *testing.T) {
	client := NewTwitchClient("example-client-id", "example-secret")
	if _, liveErr := client.IsLive(context.Background(), ""); liveErr == nil {
		t.Fatal("expected an error for an empty channel")
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Fatalf("got %q", got)
	}
	if got := truncate("0123456789abc", 10); got != "0123456789..." {
		t.Fatalf("got %q", got)
	}
}
