package register

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/chiehw/chromium-bookmarks-mcp/internal/browsers"
	"github.com/chiehw/chromium-bookmarks-mcp/internal/types"
	"github.com/chiehw/chromium-bookmarks-mcp/internal/winreg"
)

var extensionIDRe = regexp.MustCompile(`^[a-p]{32}$`)

type Manifest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Path           string   `json:"path"`
	Type           string   `json:"type"`
	AllowedOrigins []string `json:"allowed_origins"`
}

func BuildManifest(extensionID, hostPath string) Manifest {
	id := extensionID
	if id == "" {
		id = types.PublishedExtensionID
	}
	return Manifest{
		Name:           types.NativeHostName,
		Description:    "MCP Server for Chromium Bookmarks",
		Path:           hostPath,
		Type:           "stdio",
		AllowedOrigins: []string{fmt.Sprintf("chrome-extension://%s/", id)},
	}
}

func ValidateExtensionID(id string) error {
	if id != "" && !extensionIDRe.MatchString(id) {
		return fmt.Errorf("invalid extension ID: %q. Chrome extension IDs are exactly 32 lowercase letters a-p", id)
	}
	return nil
}

func Register(extensionID string) error {
	if err := ValidateExtensionID(extensionID); err != nil {
		return err
	}

	hostPath, err := installNativeHostBinary()
	if err != nil {
		return fmt.Errorf("install native host: %w", err)
	}

	bs := browsers.InstalledDefault()
	if len(bs) == 0 {
		fmt.Fprintln(os.Stderr, "No supported Chromium browsers detected.")
		fmt.Fprintln(os.Stderr, "Next step: open Chrome, Brave, Edge, or Arc at least once, then re-run `chromium-bookmarks-mcp register`.")
		return nil
	}

	manifest := BuildManifest(extensionID, hostPath)
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	filename := types.NativeHostName + ".json"

	for _, b := range bs {
		if err := os.MkdirAll(b.NativeHostDir, 0o755); err != nil {
			return err
		}
		manifestPath := filepath.Join(b.NativeHostDir, filename)
		if err := writeManifestAtomic(manifestPath, string(raw)); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Registered for %s: %s\n", b.Name, manifestPath)
		if runtime.GOOS == "windows" && b.WindowsRegistryParent != "" {
			regPath := winreg.RegistryKey(b.WindowsRegistryParent, types.NativeHostName)
			if err := winreg.Add(regPath, manifestPath); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "  Registered registry key: %s\n", regPath)
		}
	}
	return nil
}

func Unregister() error {
	bs := browsers.InstalledDefault()
	filename := types.NativeHostName + ".json"
	for _, b := range bs {
		manifestPath := filepath.Join(b.NativeHostDir, filename)
		if err := os.Remove(manifestPath); err != nil && !os.IsNotExist(err) {
			return err
		} else if err == nil {
			fmt.Fprintf(os.Stderr, "Unregistered from %s: %s\n", b.Name, manifestPath)
		}
		if runtime.GOOS == "windows" && b.WindowsRegistryParent != "" {
			regPath := winreg.RegistryKey(b.WindowsRegistryParent, types.NativeHostName)
			if err := winreg.Delete(regPath); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "  Removed registry key: %s\n", regPath)
		}
	}
	return nil
}

func writeManifestAtomic(manifestPath, contents string) error {
	tmpPath := fmt.Sprintf("%s.%d.tmp", manifestPath, os.Getpid())
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, werr := f.WriteString(contents)
	cerr := f.Close()
	if werr != nil {
		_ = os.Remove(tmpPath)
		return werr
	}
	if cerr != nil {
		_ = os.Remove(tmpPath)
		return cerr
	}
	if err := os.Rename(tmpPath, manifestPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

type BrowserRef struct {
	NativeHostDir         string
	WindowsRegistryParent string
}

type EnsureDeps struct {
	Browsers     []BrowserRef
	ExpectedPath string
	FileExists   func(string) bool
	ReadFile     func(string) (string, error)
	RegRead      func(string) (string, error)
	DoRegister   func() error
	GOOS         string
}

func allManifestsCurrent(deps EnsureDeps) bool {
	filename := types.NativeHostName + ".json"
	goos := deps.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	for _, b := range deps.Browsers {
		manifestPath := filepath.Join(b.NativeHostDir, filename)
		if !deps.FileExists(manifestPath) {
			return false
		}
		raw, err := deps.ReadFile(manifestPath)
		if err != nil {
			return false
		}
		var parsed struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			return false
		}
		if parsed.Path != deps.ExpectedPath {
			return false
		}
		if goos == "windows" && b.WindowsRegistryParent != "" {
			regPath := winreg.RegistryKey(b.WindowsRegistryParent, types.NativeHostName)
			regValue, err := deps.RegRead(regPath)
			if err != nil || regValue == "" {
				return false
			}
			if winreg.NormalizePath(regValue) != winreg.NormalizePath(manifestPath) {
				return false
			}
		}
	}
	return true
}

func EnsureWith(deps EnsureDeps) error {
	if len(deps.Browsers) == 0 {
		return nil
	}
	if allManifestsCurrent(deps) {
		return nil
	}
	return deps.DoRegister()
}

func Ensure() {
	hostPath := types.NativeHostBinaryPath()
	bs := browsers.InstalledDefault()
	refs := make([]BrowserRef, len(bs))
	for i, b := range bs {
		refs[i] = BrowserRef{NativeHostDir: b.NativeHostDir, WindowsRegistryParent: b.WindowsRegistryParent}
	}
	if err := EnsureWith(EnsureDeps{
		Browsers:     refs,
		ExpectedPath: hostPath,
		FileExists: func(p string) bool {
			_, err := os.Stat(p)
			return err == nil
		},
		ReadFile: func(p string) (string, error) {
			b, err := os.ReadFile(p)
			return string(b), err
		},
		RegRead:    winreg.Query,
		DoRegister: func() error { return Register("") },
	}); err != nil {
		fmt.Fprintf(os.Stderr, "[chromium-bookmarks-mcp] auto-register failed: %v\n", err)
	}
}
