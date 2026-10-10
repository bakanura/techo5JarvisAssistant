// Package mass is Music Assistant's own API, for what Home Assistant's integration does not pass on:
// a song's lyrics. It needs a long-lived token of a Music Assistant user made for this Show, handed to
// the device once through an action and kept on userdata. The server is the one Sendspin is paired
// with, so the address is not kept here.
package mass

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/HuskerMinion/techo5/echod/internal/layout"
)

// Port is Music Assistant's web server, where its API answers.
const Port = "8095"

// Path is where the token lives: next to the PSK, readable by root only.
var Path = filepath.Join(filepath.Dir(layout.KeyPath), "music-assistant.json")

type access struct {
	Token string `json:"token"`
}

type Client struct {
	mu    sync.Mutex
	token string
	http  *http.Client
	// port is Port except in tests, whose server listens where it likes.
	port string
}

var (
	once   sync.Once
	shared *Client
)

// Get is the client; it reads the saved token the first time.
func Get() *Client {
	once.Do(func() {
		shared = &Client{http: &http.Client{Timeout: 15 * time.Second}, port: Port}
		b, err := os.ReadFile(Path)
		if err != nil {
			return
		}
		if err := os.Chmod(Path, 0o600); err != nil {
			slog.Error("mass: refusing a saved token whose permissions cannot be secured", "err", err)
			return
		}
		var a access
		_ = json.Unmarshal(b, &a)
		shared.token = clean(a.Token)
	})
	return shared
}

// SetToken stores the token; an empty one removes it.
func (c *Client) SetToken(token string) error {
	token = clean(token)
	c.mu.Lock()
	c.token = token
	c.mu.Unlock()
	if token == "" {
		if err := os.Remove(Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	b, _ := json.Marshal(access{Token: token})
	if err := os.MkdirAll(filepath.Dir(Path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(Path, b, 0o600); err != nil {
		return err
	}
	return os.Chmod(Path, 0o600)
}

// Ready reports whether there is a token to use.
func (c *Client) Ready() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.token != ""
}

// StatusError is an answer Music Assistant gave with an HTTP error status: 401 for a token it does
// not know (revoked, or a year old), 403 for a command the Show's user may not use.
type StatusError struct {
	Command string
	Code    int
	Text    string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("mass: %s: HTTP %d %s", e.Command, e.Code, e.Text)
}

// Call runs one command on the server at host and returns its result as Music Assistant wrote it.
func (c *Client) Call(ctx context.Context, host, command string, args any) (json.RawMessage, error) {
	c.mu.Lock()
	token, port := c.token, c.port
	c.mu.Unlock()
	if token == "" {
		return nil, errors.New("mass: no token")
	}
	if host == "" {
		return nil, errors.New("mass: no server")
	}
	body, err := json.Marshal(map[string]any{"message_id": "1", "command": command, "args": args})
	if err != nil {
		return nil, err
	}
	url := "http://" + net.JoinHostPort(host, port) + "/api"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		text := strings.TrimSpace(string(out))
		if len(text) > 200 {
			text = text[:200]
		}
		return nil, &StatusError{Command: command, Code: resp.StatusCode, Text: text}
	}
	return out, nil
}

// clean takes out what a copy and paste carries along without anyone seeing it, as hass does: no
// token has a space or an invisible character in it.
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.Is(unicode.Cf, r) || !unicode.IsPrint(r) {
			return -1
		}
		return r
	}, s)
}
