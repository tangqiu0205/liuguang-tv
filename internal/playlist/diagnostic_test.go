package playlist

import (
	"context"
	"github.com/tangqiu0205/LiuguangTV/internal/diagnostic"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDiagnosticStagesAreRedacted(t *testing.T) {
	dir := t.TempDir()
	log, err := diagnostic.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/failed" {
			w.WriteHeader(403)
			return
		}
		w.Write([]byte("invalid playlist"))
	}))
	defer server.Close()
	for _, path := range []string{"/failed", "/parse"} {
		if _, err := LoadDocument(context.Background(), server.URL+path+"?token=secret-token", log); err == nil {
			t.Fatal("expected source failure")
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "diagnostic.log"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"playlist_download", "playlist_parse", "403"} {
		if !strings.Contains(string(data), required) {
			t.Fatal(string(data))
		}
	}
	if strings.Contains(string(data), "secret-token") || strings.Contains(string(data), server.URL) {
		t.Fatal("sensitive source logged")
	}
}
