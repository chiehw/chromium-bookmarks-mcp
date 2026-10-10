package register

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/chiehw/chromium-bookmarks-mcp/internal/types"
)

func TestBuildManifestDefaultOrigin(t *testing.T) {
	m := BuildManifest("", "/opt/app/native-host")
	want := "chrome-extension://" + types.PublishedExtensionID + "/"
	if len(m.AllowedOrigins) != 1 || m.AllowedOrigins[0] != want {
		t.Fatalf("origins = %v", m.AllowedOrigins)
	}
	if m.Path != "/opt/app/native-host" {
		t.Fatalf("path = %s", m.Path)
	}
	if m.Type != "stdio" {
		t.Fatalf("type = %s", m.Type)
	}
}

func TestBuildManifestOverrideID(t *testing.T) {
	id := "abcdefghijklmnopabcdefghijklmnop"
	m := BuildManifest(id, "/x")
	if m.AllowedOrigins[0] != "chrome-extension://"+id+"/" {
		t.Fatalf("origins = %v", m.AllowedOrigins)
	}
}

func TestBuildManifestNoWildcard(t *testing.T) {
	m := BuildManifest("", "/x")
	for _, o := range m.AllowedOrigins {
		if o == "chrome-extension://*/" {
			t.Fatal("wildcard origin present")
		}
	}
}

func TestValidateExtensionID(t *testing.T) {
	if err := ValidateExtensionID("short"); err == nil {
		t.Fatal("expected error for short id")
	}
	if err := ValidateExtensionID("zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"); err == nil {
		t.Fatal("expected error for letters outside a-p")
	}
	if err := ValidateExtensionID("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"); err == nil {
		t.Fatal("expected error for uppercase")
	}
	if err := ValidateExtensionID("abcdefghijklmnopabcdefghijklmnop"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateExtensionID(""); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureRegisteredIdempotency(t *testing.T) {
	tmp := t.TempDir()
	browserDir := filepath.Join(tmp, "NativeMessagingHosts")
	if err := os.MkdirAll(browserDir, 0o755); err != nil {
		t.Fatal(err)
	}
	filename := types.NativeHostName + ".json"
	manifestPath := filepath.Join(browserDir, filename)
	expectedPath := "/opt/app/native-host"
	calls := 0

	deps := func() EnsureDeps {
		return EnsureDeps{
			Browsers:     []BrowserRef{{NativeHostDir: browserDir}},
			ExpectedPath: expectedPath,
			FileExists: func(p string) bool {
				_, err := os.Stat(p)
				return err == nil
			},
			ReadFile: func(p string) (string, error) {
				b, err := os.ReadFile(p)
				return string(b), err
			},
			RegRead: func(string) (string, error) { return "", nil },
			DoRegister: func() error {
				calls++
				return nil
			},
			GOOS: "linux",
		}
	}

	seed := func(pathValue string) {
		raw, _ := json.Marshal(map[string]string{"name": types.NativeHostName, "path": pathValue})
		if err := os.WriteFile(manifestPath, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("no write when current", func(t *testing.T) {
		calls = 0
		seed(expectedPath)
		if err := EnsureWith(deps()); err != nil {
			t.Fatal(err)
		}
		if calls != 0 {
			t.Fatalf("calls = %d", calls)
		}
	})
	t.Run("re-registers when missing", func(t *testing.T) {
		calls = 0
		_ = os.Remove(manifestPath)
		if err := EnsureWith(deps()); err != nil {
			t.Fatal(err)
		}
		if calls != 1 {
			t.Fatalf("calls = %d", calls)
		}
	})
	t.Run("re-registers when stale", func(t *testing.T) {
		calls = 0
		seed("/old/stale/path/native-host")
		if err := EnsureWith(deps()); err != nil {
			t.Fatal(err)
		}
		if calls != 1 {
			t.Fatalf("calls = %d", calls)
		}
	})
	t.Run("re-registers when corrupt", func(t *testing.T) {
		calls = 0
		if err := os.WriteFile(manifestPath, []byte("{ not json"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := EnsureWith(deps()); err != nil {
			t.Fatal(err)
		}
		if calls != 1 {
			t.Fatalf("calls = %d", calls)
		}
	})
	t.Run("no register when no browsers", func(t *testing.T) {
		calls = 0
		d := deps()
		d.Browsers = nil
		if err := EnsureWith(d); err != nil {
			t.Fatal(err)
		}
		if calls != 0 {
			t.Fatalf("calls = %d", calls)
		}
	})
}
