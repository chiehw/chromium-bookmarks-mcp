// Package types holds the native-messaging wire protocol and process-wide
// constants. Keep these in sync with packages/shared (the extension).
package types

import (
	"os"
	"path/filepath"
)

type NativeMessageType string

const (
	MsgStart            NativeMessageType = "start"
	MsgServerStarted    NativeMessageType = "server_started"
	MsgCallTool         NativeMessageType = "call_tool"
	MsgCallToolResponse NativeMessageType = "call_tool_response"
	MsgPong             NativeMessageType = "pong"
)

type NativeMessage struct {
	Type                NativeMessageType `json:"type"`
	RequestID           string            `json:"requestId"`
	ResponseToRequestID string            `json:"responseToRequestId,omitempty"`
	Payload             any               `json:"payload,omitempty"`
}

type ToolCallPayload struct {
	ToolName string         `json:"toolName"`
	Args     map[string]any `json:"args"`
}

type ToolCallResponse struct {
	Status string `json:"status"`
	Data   any    `json:"data,omitempty"`
	Error  string `json:"error,omitempty"`
	Code   string `json:"code,omitempty"`
}

const (
	NativeHostName       = "com.chromium_bookmarks_mcp"
	DefaultPort          = 19420
	RequestTimeoutMS     = 30_000
	ExtensionOpTimeoutMS = 25_000
	DeadLinkDeadlineMS   = 24_000
	AuthTokenHeader      = "x-bookmarks-mcp-token"
	PublishedExtensionID = "ipcgfbbojaphhaoanjalmjmooeobjein"
	MaxPendingRequests   = 100
	MaxBufferSize        = 100 * 1024 * 1024
	ProxyFetchTimeoutMS  = 35_000
)

func CacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".cache", "chromium-bookmarks-mcp")
}

func AuthTokenPath() string {
	return filepath.Join(CacheDir(), "auth-token")
}

func NativeHostBinaryName() string {
	if os.PathSeparator == '\\' {
		return "native-host.exe"
	}
	return "native-host"
}

func NativeHostBinaryPath() string {
	return filepath.Join(CacheDir(), NativeHostBinaryName())
}
