package host

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/chiehw/chromium-bookmarks-mcp/internal/types"
)

const token = "test-token-abc"

func defaultDeps() CallToolDeps {
	return CallToolDeps{
		AuthToken:    token,
		PendingCount: func() int { return 0 },
		MaxPending:   100,
		CallTool: func(string, map[string]any) (types.ToolCallResponse, error) {
			return types.ToolCallResponse{Status: "success", Data: map[string]any{"ok": true}}, nil
		},
	}
}

func goodBody() []byte { return []byte(`{"toolName":"ping","args":{}}`) }

func TestHandleCallToolAuth(t *testing.T) {
	t.Run("absent token", func(t *testing.T) {
		body, status := HandleCallTool("", goodBody(), nil, defaultDeps())
		if status != 401 || body.Code != "UNAUTHORIZED" {
			t.Fatalf("status=%d body=%+v", status, body)
		}
	})
	t.Run("mismatched token", func(t *testing.T) {
		body, status := HandleCallTool("wrong", goodBody(), nil, defaultDeps())
		if status != 401 || body.Code != "UNAUTHORIZED" {
			t.Fatalf("status=%d body=%+v", status, body)
		}
	})
	t.Run("host has no token", func(t *testing.T) {
		d := defaultDeps()
		d.AuthToken = ""
		_, status := HandleCallTool("", goodBody(), nil, d)
		if status != 401 {
			t.Fatalf("status=%d", status)
		}
	})
	t.Run("matching token", func(t *testing.T) {
		body, status := HandleCallTool(token, goodBody(), nil, defaultDeps())
		if status != 200 || body.Status != "success" {
			t.Fatalf("status=%d body=%+v", status, body)
		}
	})
}

func TestHandleCallToolBackpressure(t *testing.T) {
	t.Run("503 at maxPending", func(t *testing.T) {
		d := defaultDeps()
		d.PendingCount = func() int { return 100 }
		d.MaxPending = 100
		body, status := HandleCallTool(token, goodBody(), nil, d)
		if status != 503 || body.Code != "BACKPRESSURE" {
			t.Fatalf("status=%d body=%+v", status, body)
		}
	})
	t.Run("does not invoke tool when backpressured", func(t *testing.T) {
		called := false
		d := defaultDeps()
		d.PendingCount = func() int { return 100 }
		d.MaxPending = 100
		d.CallTool = func(string, map[string]any) (types.ToolCallResponse, error) {
			called = true
			return types.ToolCallResponse{Status: "success"}, nil
		}
		HandleCallTool(token, goodBody(), nil, d)
		if called {
			t.Fatal("tool invoked")
		}
	})
	t.Run("passes just below the limit", func(t *testing.T) {
		d := defaultDeps()
		d.PendingCount = func() int { return 99 }
		d.MaxPending = 100
		_, status := HandleCallTool(token, goodBody(), nil, d)
		if status != 200 {
			t.Fatalf("status=%d", status)
		}
	})
}

func TestHandleCallToolBodyValidation(t *testing.T) {
	t.Run("missing toolName", func(t *testing.T) {
		body, status := HandleCallTool(token, []byte(`{"args":{}}`), nil, defaultDeps())
		if status != 400 || body.Code != "BAD_REQUEST" {
			t.Fatalf("status=%d body=%+v", status, body)
		}
	})
	t.Run("toolName not string", func(t *testing.T) {
		body, status := HandleCallTool(token, []byte(`{"toolName":123}`), nil, defaultDeps())
		if status != 400 || body.Code != "BAD_REQUEST" {
			t.Fatalf("status=%d body=%+v", status, body)
		}
	})
	t.Run("invalid JSON", func(t *testing.T) {
		body, status := HandleCallTool(token, nil, errors.New("syntax"), defaultDeps())
		if status != 400 || body.Error != "Invalid JSON body" {
			t.Fatalf("status=%d body=%+v", status, body)
		}
	})
	t.Run("defaults args to empty object", func(t *testing.T) {
		var received map[string]any
		d := defaultDeps()
		d.CallTool = func(_ string, args map[string]any) (types.ToolCallResponse, error) {
			received = args
			return types.ToolCallResponse{Status: "success"}, nil
		}
		_, status := HandleCallTool(token, []byte(`{"toolName":"ping"}`), nil, d)
		if status != 200 {
			t.Fatalf("status=%d", status)
		}
		if len(received) != 0 {
			t.Fatalf("args = %#v", received)
		}
	})
}

func TestHandleCallToolToolError(t *testing.T) {
	d := defaultDeps()
	d.CallTool = func(string, map[string]any) (types.ToolCallResponse, error) {
		return types.ToolCallResponse{}, errors.New("boom")
	}
	body, status := HandleCallTool(token, goodBody(), nil, d)
	if status != 500 || body.Code != "TOOL_ERROR" || body.Error != "boom" {
		t.Fatalf("status=%d body=%+v", status, body)
	}
}

func TestResolvePort(t *testing.T) {
	p := 31000
	if got := ResolvePort(&p); got != 31000 {
		t.Fatalf("got %d", got)
	}
	if got := ResolvePort(nil); got != types.DefaultPort {
		t.Fatalf("got %d", got)
	}
	zero := 0
	if got := ResolvePort(&zero); got != types.DefaultPort {
		t.Fatalf("zero: got %d", got)
	}
	high := 70000
	if got := ResolvePort(&high); got != types.DefaultPort {
		t.Fatalf("high: got %d", got)
	}
	neg := -1
	if got := ResolvePort(&neg); got != types.DefaultPort {
		t.Fatalf("neg: got %d", got)
	}
}

func TestDispatchToolCallLifecycle(t *testing.T) {
	t.Run("registers then resolves", func(t *testing.T) {
		m := map[string]*Pending{}
		done := make(chan types.ToolCallResponse, 1)
		go func() {
			resp, err := DispatchToolCall(m, "req-1", func() {}, time.Second)
			if err != nil {
				t.Errorf("err %v", err)
				return
			}
			done <- resp
		}()
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			if _, ok := m["req-1"]; ok {
				break
			}
			time.Sleep(time.Millisecond)
		}
		p, ok := m["req-1"]
		if !ok {
			t.Fatal("entry not registered")
		}
		p.Timer.Stop()
		delete(m, "req-1")
		want := types.ToolCallResponse{Status: "success", Data: 42}
		p.Ch <- callResult{resp: want}
		got := <-done
		if got.Status != "success" {
			t.Fatalf("got %+v", got)
		}
		raw, _ := json.Marshal(got.Data)
		if string(raw) != "42" {
			t.Fatalf("data %s", raw)
		}
		if _, leaked := m["req-1"]; leaked {
			t.Fatal("entry leaked")
		}
	})
	t.Run("timeout deletes entry", func(t *testing.T) {
		m := map[string]*Pending{}
		_, err := DispatchToolCall(m, "req-timeout", func() {}, 5*time.Millisecond)
		if err == nil || err.Error() != "request timed out after 5ms" {
			t.Fatalf("err = %v", err)
		}
		if _, ok := m["req-timeout"]; ok {
			t.Fatal("entry leaked")
		}
	})
	t.Run("default timeout is innermost budget", func(t *testing.T) {
		if types.ExtensionOpTimeoutMS != 25_000 {
			t.Fatalf("budget = %d", types.ExtensionOpTimeoutMS)
		}
	})
}
