package store

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
)

func TestSubscriptionErrorDetailsAndRedaction(t *testing.T) {
	s := State{}
	id, err := s.UpsertSubscription("", "Test", "https://example.org/list")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		err  error
		want string
	}{
		{fmt.Errorf("HTTP 403"), "HTTP 403"},
		{fmt.Errorf("节目单超过 16 MiB"), "节目单超过 16 MiB"},
		{fmt.Errorf("下载失败: %w", &url.Error{Op: "Get", URL: "https://user:secret@example.org/list?token=private", Err: context.DeadlineExceeded}), context.DeadlineExceeded.Error()},
		{&os.PathError{Op: "open", Path: "C:/private/list.m3u", Err: os.ErrNotExist}, os.ErrNotExist.Error()},
	} {
		if !s.UpdateSubscriptionCache(id, 1, nil, tc.err, time.Now()) {
			t.Fatal("refresh rejected")
		}
		message := s.FindSubscription(id).LastError
		if !strings.Contains(message, tc.want) || strings.Contains(message, "private") || strings.Contains(message, "secret") {
			t.Fatalf("missing reason or leaked sensitive data: %q", message)
		}
	}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := Save(path, s); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.FindSubscription(id).LastError != s.FindSubscription(id).LastError {
		t.Fatal("error detail lost on reload")
	}
}

func TestHTMLRefreshPreservesCache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("<!DOCTYPE html>\n<html>Login</html>"))
	}))
	defer server.Close()
	s := State{}
	id, err := s.UpsertSubscription("", "Test", server.URL)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().Add(-time.Hour)
	cached := []playlist.Channel{{Name: "Cached", URL: "https://example.org/live"}}
	s.ApplyRefresh(id, 1, cached, nil, at)
	channels, loadErr := playlist.Load(context.Background(), server.URL)
	if loadErr == nil {
		t.Fatal("HTML accepted")
	}
	s.ApplyRefresh(id, 1, channels, loadErr, time.Now())
	sub := s.FindSubscription(id)
	if len(sub.Channels) != 1 || sub.Channels[0].URL != cached[0].URL || !sub.UpdatedAt.Equal(at) || !strings.Contains(sub.LastError, "网页") {
		t.Fatalf("cache or diagnostic incorrect: %+v", sub)
	}
}
