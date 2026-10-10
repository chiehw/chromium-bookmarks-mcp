package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chiehw/chromium-bookmarks-mcp/internal/host"
	"github.com/chiehw/chromium-bookmarks-mcp/internal/proxy"
	"github.com/chiehw/chromium-bookmarks-mcp/internal/register"
	"github.com/chiehw/chromium-bookmarks-mcp/internal/version"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Fatal: %v\n", err)
		os.Exit(1)
	}
}

func isNativeHost() bool {
	base := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
	if base == "native-host" {
		return true
	}
	return len(os.Args) > 1 && os.Args[1] == "native-host"
}

func run() error {
	if isNativeHost() {
		return host.Run()
	}

	cmd := ""
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	switch cmd {
	case "register":
		id := ""
		if len(os.Args) > 2 {
			id = os.Args[2]
		}
		return register.Register(id)
	case "unregister":
		return register.Unregister()
	case "doctor":
		return register.Doctor()
	case "version", "-version", "--version", "-v":
		fmt.Println(version.Version)
		return nil
	case "", "serve":
		return proxy.Run()
	default:
		fmt.Print(`Usage: chromium-bookmarks-mcp [command]

Commands:
  (none)                   Start MCP stdio proxy (default)
  register [extension-id]  Register native host for all detected browsers
                           (optional extension-id overrides the published ID)
  unregister               Remove native host registration
  doctor                   Diagnose connection issues
  version                  Print version
`)
		return nil
	}
}
