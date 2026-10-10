package winreg

import "testing"

func TestNormalizePath(t *testing.T) {
	got := NormalizePath("  C:\\foo\\bar\\  ")
	if got == "" {
		t.Fatal("empty")
	}
	if got[len(got)-1] == '\\' || got[len(got)-1] == '/' {
		t.Fatalf("trailing sep: %q", got)
	}
}

func TestRegistryKey(t *testing.T) {
	got := RegistryKey(`Google\Chrome`, "com.chromium_bookmarks_mcp")
	want := `HKCU\Software\Google\Chrome\NativeMessagingHosts\com.chromium_bookmarks_mcp`
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}
