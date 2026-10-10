package host

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/chiehw/chromium-bookmarks-mcp/internal/protocol"
	"github.com/chiehw/chromium-bookmarks-mcp/internal/types"
	"github.com/google/uuid"
)

type callResult struct {
	resp types.ToolCallResponse
	err  error
}

type pendingCall struct {
	ch    chan callResult
	timer *time.Timer
}

type Host struct {
	authToken string
	in        io.Reader
	out       io.Writer

	mu      sync.Mutex
	pending map[string]*pendingCall
	started bool

	logFile string
}

func New(authToken string, in io.Reader, out io.Writer) *Host {
	return &Host{
		authToken: authToken,
		in:        in,
		out:       out,
		pending:   make(map[string]*pendingCall),
		logFile:   os.Getenv("BOOKMARKS_MCP_LOG"),
	}
}

func InitAuthToken() (string, error) {
	token := uuid.NewString()
	path := types.AuthTokenPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(token), 0o600); err != nil {
		return "", err
	}
	return token, nil
}

func Run() error {
	token, err := InitAuthToken()
	if err != nil {
		return err
	}
	return New(token, os.Stdin, os.Stdout).Serve()
}

func (h *Host) log(line string) {
	fmt.Fprintln(os.Stderr, line)
	if h.logFile == "" {
		return
	}
	f, err := os.OpenFile(h.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(f, "%s %s\n", time.Now().UTC().Format(time.RFC3339Nano), line)
	_ = f.Close()
}

var stdoutMu sync.Mutex

func (h *Host) send(msg types.NativeMessage) error {
	encoded, err := protocol.Encode(msg)
	if err != nil {
		return err
	}
	stdoutMu.Lock()
	defer stdoutMu.Unlock()
	_, err = h.out.Write(encoded)
	return err
}

func (h *Host) Serve() error {
	var buf []byte
	tmp := make([]byte, 64*1024)
	for {
		n, err := h.in.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			if len(buf) > types.MaxBufferSize {
				h.log(fmt.Sprintf("Buffer overflow: %d bytes, resetting.", len(buf)))
				buf = nil
				continue
			}
			messages, remaining, decErr := protocol.Decode(buf)
			if decErr != nil {
				h.log(fmt.Sprintf("Native protocol decode error: %v", decErr))
				buf = nil
				continue
			}
			buf = remaining
			for _, raw := range messages {
				h.handleExtensionMessage(raw)
			}
		}
		if errors.Is(err, io.EOF) {
			h.log("Extension disconnected, shutting down.")
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func (h *Host) handleExtensionMessage(raw any) {
	b, err := json.Marshal(raw)
	if err != nil {
		return
	}
	var msg types.NativeMessage
	if err := json.Unmarshal(b, &msg); err != nil {
		return
	}

	switch msg.Type {
	case types.MsgStart:
		var payload struct {
			Port *float64 `json:"port"`
		}
		if msg.Payload != nil {
			pb, _ := json.Marshal(msg.Payload)
			_ = json.Unmarshal(pb, &payload)
		}
		var port *int
		if payload.Port != nil {
			p := int(*payload.Port)
			if float64(p) == *payload.Port {
				port = &p
			}
		}
		h.startHTTP(port)
	case types.MsgCallToolResponse:
		if msg.ResponseToRequestID == "" {
			return
		}
		h.mu.Lock()
		p, ok := h.pending[msg.ResponseToRequestID]
		if ok {
			delete(h.pending, msg.ResponseToRequestID)
		}
		h.mu.Unlock()
		if !ok {
			return
		}
		p.timer.Stop()
		var resp types.ToolCallResponse
		if msg.Payload != nil {
			pb, _ := json.Marshal(msg.Payload)
			_ = json.Unmarshal(pb, &resp)
		}
		p.ch <- callResult{resp: resp}
	case types.MsgPong:
	}
}

func ResolvePort(payloadPort *int) int {
	if payloadPort != nil && *payloadPort >= 1 && *payloadPort <= 65535 {
		return *payloadPort
	}
	return types.DefaultPort
}

type CallToolDeps struct {
	AuthToken    string
	PendingCount func() int
	MaxPending   int
	CallTool     func(toolName string, args map[string]any) (types.ToolCallResponse, error)
}

func HandleCallTool(headerToken string, body []byte, bodyErr error, deps CallToolDeps) (types.ToolCallResponse, int) {
	if deps.AuthToken == "" || headerToken != deps.AuthToken {
		return types.ToolCallResponse{Status: "error", Error: "unauthorized", Code: "UNAUTHORIZED"}, 401
	}
	if deps.PendingCount() >= deps.MaxPending {
		return types.ToolCallResponse{Status: "error", Error: "Too many pending requests", Code: "BACKPRESSURE"}, 503
	}
	if bodyErr != nil {
		return types.ToolCallResponse{Status: "error", Error: "Invalid JSON body", Code: "BAD_REQUEST"}, 400
	}
	var parsed struct {
		ToolName any            `json:"toolName"`
		Args     map[string]any `json:"args"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return types.ToolCallResponse{Status: "error", Error: "Invalid JSON body", Code: "BAD_REQUEST"}, 400
	}
	toolName, ok := parsed.ToolName.(string)
	if !ok || toolName == "" {
		return types.ToolCallResponse{Status: "error", Error: "toolName (string) is required", Code: "BAD_REQUEST"}, 400
	}
	args := parsed.Args
	if args == nil {
		args = map[string]any{}
	}
	result, err := deps.CallTool(toolName, args)
	if err != nil {
		return types.ToolCallResponse{Status: "error", Error: err.Error(), Code: "TOOL_ERROR"}, 500
	}
	return result, 200
}

// Pending is the test-facing pending-request entry (mirrors the TS PendingRequest).
type Pending struct {
	Ch    chan callResult
	Timer *time.Timer
}

// DispatchToolCall registers a pending entry, arms a timeout that deletes it,
// and fires send. Whoever successfully removes the map entry owns the result.
func DispatchToolCall(m map[string]*Pending, requestID string, send func(), timeout time.Duration) (types.ToolCallResponse, error) {
	if timeout == 0 {
		timeout = time.Duration(types.ExtensionOpTimeoutMS) * time.Millisecond
	}
	p := &Pending{Ch: make(chan callResult, 1)}
	p.Timer = time.AfterFunc(timeout, func() {
		if _, ok := m[requestID]; ok {
			delete(m, requestID)
			p.Ch <- callResult{err: fmt.Errorf("request timed out after %dms", timeout.Milliseconds())}
		}
	})
	m[requestID] = p
	send()
	r := <-p.Ch
	return r.resp, r.err
}

func (h *Host) callExtensionTool(toolName string, args map[string]any) (types.ToolCallResponse, error) {
	requestID := uuid.NewString()
	timeout := time.Duration(types.ExtensionOpTimeoutMS) * time.Millisecond
	p := &pendingCall{ch: make(chan callResult, 1)}
	p.timer = time.AfterFunc(timeout, func() {
		h.mu.Lock()
		_, ok := h.pending[requestID]
		if ok {
			delete(h.pending, requestID)
		}
		h.mu.Unlock()
		if ok {
			p.ch <- callResult{err: fmt.Errorf("request timed out after %dms", types.ExtensionOpTimeoutMS)}
		}
	})
	h.mu.Lock()
	h.pending[requestID] = p
	h.mu.Unlock()

	if err := h.send(types.NativeMessage{
		Type:      types.MsgCallTool,
		RequestID: requestID,
		Payload:   types.ToolCallPayload{ToolName: toolName, Args: args},
	}); err != nil {
		h.mu.Lock()
		if cur, ok := h.pending[requestID]; ok && cur == p {
			delete(h.pending, requestID)
		}
		h.mu.Unlock()
		p.timer.Stop()
		return types.ToolCallResponse{}, err
	}

	r := <-p.ch
	return r.resp, r.err
}

func (h *Host) startHTTP(payloadPort *int) {
	h.mu.Lock()
	if h.started {
		h.mu.Unlock()
		return
	}
	h.started = true
	h.mu.Unlock()

	port := ResolvePort(payloadPort)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if isAddrInUse(err) {
			if isOurHealthyInstance(port) {
				h.log(fmt.Sprintf("Port %d already serving a healthy chromium-bookmarks-mcp instance; deferring to it.", port))
				_ = h.send(types.NativeMessage{
					Type:      types.MsgServerStarted,
					RequestID: uuid.NewString(),
					Payload:   map[string]any{"port": port},
				})
				return
			}
			h.log(fmt.Sprintf("Port %d is in use by another process. Close the other program (or the other Chromium browser) using 127.0.0.1:%d and reconnect.", port, port))
			os.Exit(1)
		}
		h.log(fmt.Sprintf("Failed to start HTTP server on 127.0.0.1:%d: %v", port, err))
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		n := len(h.pending)
		h.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "pendingRequests": n})
	})
	mux.HandleFunc("POST /call-tool", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<20))
		resp, status := HandleCallTool(
			r.Header.Get(types.AuthTokenHeader),
			body,
			err,
			CallToolDeps{
				AuthToken:    h.authToken,
				PendingCount: func() int { h.mu.Lock(); defer h.mu.Unlock(); return len(h.pending) },
				MaxPending:   types.MaxPendingRequests,
				CallTool:     h.callExtensionTool,
			},
		)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(resp)
	})

	go func() {
		srv := &http.Server{Handler: mux}
		_ = srv.Serve(ln)
	}()

	_ = h.send(types.NativeMessage{
		Type:      types.MsgServerStarted,
		RequestID: uuid.NewString(),
		Payload:   map[string]any{"port": port},
	})
	h.log(fmt.Sprintf("HTTP server listening on 127.0.0.1:%d", port))
}

func isAddrInUse(err error) bool {
	if errors.Is(err, syscall.EADDRINUSE) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "address already in use") ||
		strings.Contains(msg, "only one usage of each socket address")
}

func isOurHealthyInstance(port int) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	res, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err != nil {
		return false
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return false
	}
	var body struct {
		Status          any `json:"status"`
		PendingRequests any `json:"pendingRequests"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return false
	}
	status, _ := body.Status.(string)
	_, isNum := body.PendingRequests.(float64)
	return status == "ok" && isNum
}
