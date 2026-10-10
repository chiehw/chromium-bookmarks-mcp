package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/chiehw/chromium-bookmarks-mcp/internal/register"
	"github.com/chiehw/chromium-bookmarks-mcp/internal/types"
	"github.com/chiehw/chromium-bookmarks-mcp/internal/version"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const notConnectedMsg = "Extension not connected. Please open your browser and ensure the Chromium Bookmarks MCP extension is installed."

type HTTPError struct {
	Status     int
	StatusText string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.Status, e.StatusText)
}

func IsNotConnectedError(err error) bool {
	if err == nil {
		return false
	}
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return IsNotConnectedError(urlErr.Err)
	}
	if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) {
		return true
	}
	return false
}

var statusCodeMap = map[int]string{
	400: "BAD_REQUEST",
	401: "UNAUTHORIZED",
	503: "BACKPRESSURE",
}

func UnwrapNonOKBody(status int, contentType string, body []byte) (types.ToolCallResponse, error) {
	if strings.Contains(contentType, "application/json") {
		var parsed types.ToolCallResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return types.ToolCallResponse{}, &HTTPError{Status: status, StatusText: "Invalid JSON"}
		}
		parsed.Status = "error"
		if parsed.Code == "" {
			if c, ok := statusCodeMap[status]; ok {
				parsed.Code = c
			} else {
				parsed.Code = "TOOL_ERROR"
			}
		}
		return parsed, nil
	}
	return types.ToolCallResponse{}, &HTTPError{Status: status, StatusText: http.StatusText(status)}
}

type Client struct {
	HTTP      *http.Client
	BaseURL   string
	TokenPath string
	DebugJSON bool
}

func NewClient() *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: time.Duration(types.ProxyFetchTimeoutMS) * time.Millisecond},
		BaseURL:   fmt.Sprintf("http://127.0.0.1:%d", types.DefaultPort),
		TokenPath: types.AuthTokenPath(),
		DebugJSON: os.Getenv("BOOKMARKS_MCP_DEBUG") == "1",
	}
}

func (c *Client) readAuthToken() string {
	b, err := os.ReadFile(c.TokenPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func (c *Client) fetchCallTool(toolName string, args map[string]any, token string) (*http.Response, error) {
	payload, err := json.Marshal(map[string]any{"toolName": toolName, "args": args})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/call-tool", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set(types.AuthTokenHeader, token)
	}
	return c.HTTP.Do(req)
}

func (c *Client) CallNativeHost(toolName string, args map[string]any) (types.ToolCallResponse, error) {
	res, err := c.fetchCallTool(toolName, args, c.readAuthToken())
	if err != nil {
		return types.ToolCallResponse{}, err
	}
	if res.StatusCode == 401 {
		_ = res.Body.Close()
		res, err = c.fetchCallTool(toolName, args, c.readAuthToken())
		if err != nil {
			return types.ToolCallResponse{}, err
		}
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return types.ToolCallResponse{}, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return UnwrapNonOKBody(res.StatusCode, res.Header.Get("Content-Type"), body)
	}
	var parsed types.ToolCallResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return types.ToolCallResponse{}, err
	}
	return parsed, nil
}

func (c *Client) handleTool(ctx context.Context, name string, args any) (*mcp.CallToolResult, any, error) {
	raw, err := json.Marshal(args)
	if err != nil {
		return errorResult(fmt.Sprintf("Error: %v", err)), nil, nil
	}
	m := map[string]any{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return errorResult(fmt.Sprintf("Error: %v", err)), nil, nil
	}
	if m == nil {
		m = map[string]any{}
	}
	result, err := c.CallNativeHost(name, m)
	if err != nil {
		if IsNotConnectedError(err) {
			return errorResult(notConnectedMsg + " [NOT_CONNECTED]"), nil, nil
		}
		return errorResult(fmt.Sprintf("Error: %v", err)), nil, nil
	}
	if result.Status == "error" {
		code := ""
		if result.Code != "" {
			code = " [" + result.Code + "]"
		}
		return errorResult(fmt.Sprintf("Error%s: %s", code, result.Error)), nil, nil
	}
	var text []byte
	if c.DebugJSON {
		text, _ = json.MarshalIndent(result.Data, "", "  ")
	} else {
		text, _ = json.Marshal(result.Data)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(text)}},
	}, nil, nil
}

func errorResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
		IsError: true,
	}
}

func Run() error {
	register.Ensure()
	c := NewClient()
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "chromium-bookmarks-mcp",
		Version: version.Version,
	}, &mcp.ServerOptions{
		Instructions: "Manage Chromium browser bookmarks. Requires the Chromium Bookmarks MCP extension to be installed and the browser to be open.",
	})
	addTools(server, c)
	fmt.Fprintln(os.Stderr, "MCP stdio proxy started.")
	return server.Run(context.Background(), &mcp.StdioTransport{})
}
