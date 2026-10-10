package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/chiehw/chromium-bookmarks-mcp/internal/types"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAddToolsRegisters(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	addTools(server, NewClient())
}

func TestIsNotConnectedError(t *testing.T) {
	t.Run("ECONNREFUSED", func(t *testing.T) {
		err := &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}
		if !IsNotConnectedError(err) {
			t.Fatal("expected true")
		}
	})
	t.Run("timeout", func(t *testing.T) {
		if !IsNotConnectedError(context.DeadlineExceeded) {
			t.Fatal("expected true")
		}
	})
	t.Run("canceled", func(t *testing.T) {
		if !IsNotConnectedError(context.Canceled) {
			t.Fatal("expected true")
		}
	})
	t.Run("HttpError is never not-connected", func(t *testing.T) {
		if IsNotConnectedError(&HTTPError{Status: 500, StatusText: "Internal Server Error"}) {
			t.Fatal("HttpError must not classify as not-connected")
		}
	})
	t.Run("substring in message is not structural", func(t *testing.T) {
		err := errors.New("the tool reported ECONNREFUSED in its output")
		if IsNotConnectedError(err) {
			t.Fatal("must not classify by substring")
		}
	})
}

func TestUnwrapNonOKBody(t *testing.T) {
	t.Run("503 BACKPRESSURE", func(t *testing.T) {
		raw, _ := json.Marshal(map[string]any{"status": "error", "error": "Too many pending requests"})
		out, err := UnwrapNonOKBody(503, "application/json", raw)
		if err != nil {
			t.Fatal(err)
		}
		if out.Status != "error" || out.Code != "BACKPRESSURE" || out.Error != "Too many pending requests" {
			t.Fatalf("%+v", out)
		}
	})
	t.Run("400 BAD_REQUEST", func(t *testing.T) {
		raw, _ := json.Marshal(map[string]any{"status": "error", "error": "Invalid JSON body"})
		out, err := UnwrapNonOKBody(400, "application/json", raw)
		if err != nil {
			t.Fatal(err)
		}
		if out.Code != "BAD_REQUEST" {
			t.Fatalf("%+v", out)
		}
	})
	t.Run("401 UNAUTHORIZED", func(t *testing.T) {
		raw, _ := json.Marshal(map[string]any{"status": "error", "error": "unauthorized"})
		out, err := UnwrapNonOKBody(401, "application/json", raw)
		if err != nil {
			t.Fatal(err)
		}
		if out.Code != "UNAUTHORIZED" {
			t.Fatalf("%+v", out)
		}
	})
	t.Run("500 default TOOL_ERROR", func(t *testing.T) {
		raw, _ := json.Marshal(map[string]any{"status": "error", "error": "boom"})
		out, err := UnwrapNonOKBody(500, "application/json", raw)
		if err != nil {
			t.Fatal(err)
		}
		if out.Code != "TOOL_ERROR" || out.Error != "boom" {
			t.Fatalf("%+v", out)
		}
	})
	t.Run("host-supplied code preserved", func(t *testing.T) {
		raw, _ := json.Marshal(map[string]any{"status": "error", "error": "x", "code": "TIMEOUT"})
		out, err := UnwrapNonOKBody(500, "application/json", raw)
		if err != nil {
			t.Fatal(err)
		}
		if out.Code != "TIMEOUT" {
			t.Fatalf("%+v", out)
		}
	})
	t.Run("non-JSON throws HTTPError", func(t *testing.T) {
		_, err := UnwrapNonOKBody(502, "text/html", []byte("<html>"))
		var httpErr *HTTPError
		if !errors.As(err, &httpErr) {
			t.Fatalf("got %v", err)
		}
	})
}

func newTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &Client{
		HTTP:      srv.Client(),
		BaseURL:   srv.URL,
		TokenPath: filepath.Join(t.TempDir(), "auth-token"),
	}
}

func TestCallNativeHost(t *testing.T) {
	t.Run("200 returns parsed body", func(t *testing.T) {
		c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"ok": 1}})
		}))
		out, err := c.CallNativeHost("ping", map[string]any{})
		if err != nil {
			t.Fatal(err)
		}
		if out.Status != "success" {
			t.Fatalf("%+v", out)
		}
	})
	t.Run("500 returns unwrapped error", func(t *testing.T) {
		c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(500)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "error", "error": "handler exploded"})
		}))
		out, err := c.CallNativeHost("bookmark_delete", map[string]any{"id": "9"})
		if err != nil {
			t.Fatal(err)
		}
		if out.Status != "error" || out.Error != "handler exploded" || out.Code != "TOOL_ERROR" {
			t.Fatalf("%+v", out)
		}
	})
	t.Run("503 BACKPRESSURE", func(t *testing.T) {
		c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(503)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "error", "error": "Too many pending requests"})
		}))
		out, err := c.CallNativeHost("ping", map[string]any{})
		if err != nil {
			t.Fatal(err)
		}
		if out.Code != "BACKPRESSURE" {
			t.Fatalf("%+v", out)
		}
	})
	t.Run("ECONNREFUSED propagates as not-connected", func(t *testing.T) {
		c := &Client{
			HTTP:    &http.Client{},
			BaseURL: "http://127.0.0.1:1",
		}
		_, err := c.CallNativeHost("ping", map[string]any{})
		if err == nil {
			t.Fatal("expected error")
		}
		if !IsNotConnectedError(err) {
			t.Fatalf("not classified: %v", err)
		}
	})
}

func TestAuthTokenHeader(t *testing.T) {
	t.Run("sends header when token file present", func(t *testing.T) {
		var seen string
		c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seen = r.Header.Get(types.AuthTokenHeader)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{}})
		}))
		if err := os.WriteFile(c.TokenPath, []byte("abc-token"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := c.CallNativeHost("ping", map[string]any{}); err != nil {
			t.Fatal(err)
		}
		if seen != "abc-token" {
			t.Fatalf("token header = %q", seen)
		}
	})
	t.Run("401 retries once", func(t *testing.T) {
		calls := 0
		c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.Header().Set("Content-Type", "application/json")
			if calls == 1 {
				w.WriteHeader(401)
				_ = json.NewEncoder(w).Encode(map[string]any{"status": "error", "error": "unauthorized", "code": "UNAUTHORIZED"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"retried": true}})
		}))
		out, err := c.CallNativeHost("ping", map[string]any{})
		if err != nil {
			t.Fatal(err)
		}
		if calls != 2 {
			t.Fatalf("calls = %d", calls)
		}
		if out.Status != "success" {
			t.Fatalf("%+v", out)
		}
	})
	t.Run("persistent 401 stops after one retry", func(t *testing.T) {
		calls := 0
		c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(401)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "error", "error": "unauthorized"})
		}))
		out, err := c.CallNativeHost("ping", map[string]any{})
		if err != nil {
			t.Fatal(err)
		}
		if calls != 2 {
			t.Fatalf("calls = %d", calls)
		}
		if out.Status != "error" || out.Code != "UNAUTHORIZED" {
			t.Fatalf("%+v", out)
		}
	})
}

func TestHandleToolNotConnected(t *testing.T) {
	c := &Client{
		HTTP:    &http.Client{},
		BaseURL: "http://127.0.0.1:1",
	}
	res, _, err := c.handleTool(context.Background(), "ping", pingArgs{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("expected isError")
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content type %T", res.Content[0])
	}
	if !strings.Contains(text.Text, "NOT_CONNECTED") {
		t.Fatalf("text = %s", text.Text)
	}
}
