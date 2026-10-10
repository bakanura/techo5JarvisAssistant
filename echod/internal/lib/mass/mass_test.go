package mass

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testClient(t *testing.T, h http.HandlerFunc) (*Client, string) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	host, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	Path = filepath.Join(t.TempDir(), "music-assistant.json")
	return &Client{http: &http.Client{Timeout: 5 * time.Second}, port: port}, host
}

func TestCallSendsTheCommandWithTheToken(t *testing.T) {
	c, host := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api" {
			http.Error(w, "wrong place", http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer secret-token" {
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}
		var msg struct {
			Command string         `json:"command"`
			Args    map[string]any `json:"args"`
		}
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &msg); err != nil || msg.Command != "music/item_by_uri" || msg.Args["uri"] != "library://track/1" {
			http.Error(w, "bad body "+string(b), http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"item_id": "1", "provider": "library"}`))
	})
	if _, err := c.Call(context.Background(), host, "x", nil); err == nil {
		t.Fatal("a call without a token must fail before it is sent")
	}
	if err := c.SetToken(" secret-token​\n"); err != nil {
		t.Fatal(err)
	}
	out, err := c.Call(context.Background(), host, "music/item_by_uri", map[string]any{"uri": "library://track/1"})
	if err != nil || string(out) != `{"item_id": "1", "provider": "library"}` {
		t.Fatalf("got %s, %v", out, err)
	}
}

func TestCallReportsTheStatus(t *testing.T) {
	c, host := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "This command requires the library.read scope", http.StatusForbidden)
	})
	_ = c.SetToken("t")
	_, err := c.Call(context.Background(), host, "metadata/get_track_lyrics", nil)
	var se *StatusError
	if !errors.As(err, &se) || se.Code != http.StatusForbidden || se.Command != "metadata/get_track_lyrics" {
		t.Fatalf("got %v", err)
	}
	if _, err := c.Call(context.Background(), "", "x", nil); err == nil {
		t.Fatal("no server is an error")
	}
}

func TestTokenIsKeptPrivateAndRemoved(t *testing.T) {
	c, _ := testClient(t, nil)
	if err := c.SetToken("abc"); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(Path)
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("token file: %v, %v", st, err)
	}
	b, _ := os.ReadFile(Path)
	if string(b) != `{"token":"abc"}` || !c.Ready() {
		t.Fatalf("saved %s", b)
	}
	if err := c.SetToken(""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(Path); !os.IsNotExist(err) || c.Ready() {
		t.Fatalf("the token is still there: %v", err)
	}
	if err := c.SetToken(""); err != nil {
		t.Fatalf("removing no token: %v", err)
	}
}
