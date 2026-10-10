package register

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/chiehw/chromium-bookmarks-mcp/internal/browsers"
	"github.com/chiehw/chromium-bookmarks-mcp/internal/types"
	"github.com/chiehw/chromium-bookmarks-mcp/internal/winreg"
)

func Doctor() error {
	fmt.Println("=== chromium-bookmarks-mcp doctor ===")
	fmt.Println()

	hostPath := types.NativeHostBinaryPath()
	_, err := os.Stat(hostPath)
	hostExists := err == nil
	fmt.Printf("Native host binary: %s\n", hostPath)
	if hostExists {
		fmt.Println("  Exists: YES")
	} else {
		fmt.Println("  Exists: NO")
	}

	bs := browsers.InstalledDefault()
	filename := types.NativeHostName + ".json"
	fmt.Printf("\nDetected browsers: %d\n", len(bs))
	for _, b := range bs {
		manifestPath := filepath.Join(b.NativeHostDir, filename)
		_, merr := os.Stat(manifestPath)
		fileOk := merr == nil
		var status string
		if runtime.GOOS == "windows" && b.WindowsRegistryParent != "" {
			regPath := winreg.RegistryKey(b.WindowsRegistryParent, types.NativeHostName)
			regValue, qerr := winreg.Query(regPath)
			regOk := qerr == nil && regValue != "" && winreg.NormalizePath(regValue) == winreg.NormalizePath(manifestPath)
			switch {
			case fileOk && regOk:
				status = "REGISTERED"
			case fileOk && !regOk:
				status = "MANIFEST OK / REGISTRY MISSING (run register)"
			case !fileOk && regOk:
				status = "REGISTRY OK / MANIFEST MISSING (run register)"
			default:
				status = "NOT REGISTERED"
			}
		} else if fileOk {
			status = "REGISTERED"
		} else {
			status = "NOT REGISTERED"
		}
		fmt.Printf("  %s: %s\n", b.Name, status)
		if fileOk {
			origins := readManifestOrigins(manifestPath)
			if origins == nil {
				fmt.Println("    allowed_origins: (unreadable)")
			} else {
				fmt.Printf("    allowed_origins: %s\n", joinComma(origins))
			}
		}
	}
	fmt.Printf("\nExpected extension ID: %s\n", types.PublishedExtensionID)
	fmt.Println("  (allowed_origins above must contain chrome-extension://<this id>/ to connect)")

	fmt.Println("\nHTTP server connectivity:")
	client := &http.Client{Timeout: 2 * time.Second}
	res, herr := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", types.DefaultPort))
	if herr == nil {
		defer res.Body.Close()
		var data any
		_ = json.NewDecoder(res.Body).Decode(&data)
		raw, _ := json.Marshal(data)
		fmt.Printf("  Status: CONNECTED — %s\n", raw)
		return nil
	}

	switch probePort(types.DefaultPort) {
	case "occupied":
		fmt.Printf("  Status: PORT %d OCCUPIED but /health failed — another process holds the port, or a second browser is already connected.\n", types.DefaultPort)
	default:
		fmt.Printf("  Status: NOT RUNNING (port %d is free — open browser and click the extension icon to activate)\n", types.DefaultPort)
	}
	return nil
}

func readManifestOrigins(manifestPath string) []string {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil
	}
	var parsed struct {
		AllowedOrigins []string `json:"allowed_origins"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil
	}
	return parsed.AllowedOrigins
}

func probePort(port int) string {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 1500*time.Millisecond)
	if err != nil {
		return "free"
	}
	_ = conn.Close()
	return "occupied"
}

func joinComma(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}
