package browsers

import (
	"path/filepath"
	"testing"
)

const home = "/home/tester"
const winHome = `C:\Users\tester`

func envWith(osName, homeDir, localAppData string, present []string) Env {
	set := map[string]struct{}{}
	for _, p := range present {
		set[p] = struct{}{}
	}
	return Env{
		GOOS:         osName,
		Home:         homeDir,
		LocalAppData: localAppData,
		Exists: func(p string) bool {
			_, ok := set[p]
			return ok
		},
	}
}

func names(bs []Info) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = b.Name
	}
	return out
}

func find(bs []Info, name string) Info {
	for _, b := range bs {
		if b.Name == name {
			return b
		}
	}
	return Info{}
}

func TestMacOSCandidates(t *testing.T) {
	got := Candidates(envWith("darwin", home, "", nil))
	if want := []string{"Chrome", "Brave", "Edge", "Arc", "Chromium"}; !equal(names(got), want) {
		t.Fatalf("names = %v, want %v", names(got), want)
	}
	base := filepath.Join(home, "Library", "Application Support")
	if find(got, "Chrome").NativeHostDir != filepath.Join(base, "Google", "Chrome", "NativeMessagingHosts") {
		t.Fatalf("chrome dir = %s", find(got, "Chrome").NativeHostDir)
	}
	if find(got, "Arc").NativeHostDir != filepath.Join(base, "Arc", "User Data", "NativeMessagingHosts") {
		t.Fatalf("arc dir = %s", find(got, "Arc").NativeHostDir)
	}
	for _, b := range got {
		if b.WindowsRegistryParent != "" {
			t.Fatalf("unexpected registry parent on darwin: %+v", b)
		}
	}
}

func TestLinuxCandidates(t *testing.T) {
	got := Candidates(envWith("linux", home, "", nil))
	if want := []string{"Chrome", "Brave", "Edge", "Chromium"}; !equal(names(got), want) {
		t.Fatalf("names = %v, want %v", names(got), want)
	}
	if find(got, "Chrome").NativeHostDir != filepath.Join(home, ".config", "google-chrome", "NativeMessagingHosts") {
		t.Fatalf("chrome dir = %s", find(got, "Chrome").NativeHostDir)
	}
	if find(got, "Brave").NativeHostDir != filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser", "NativeMessagingHosts") {
		t.Fatalf("brave dir = %s", find(got, "Brave").NativeHostDir)
	}
}

func TestWindowsCandidatesUseLocalAppData(t *testing.T) {
	local := `D:\Custom\Local`
	got := Candidates(envWith("windows", winHome, local, nil))
	if want := []string{"Chrome", "Brave", "Edge"}; !equal(names(got), want) {
		t.Fatalf("names = %v, want %v", names(got), want)
	}
	if find(got, "Chrome").NativeHostDir != filepath.Join(local, "Google", "Chrome", "User Data", "NativeMessagingHosts") {
		t.Fatalf("chrome dir = %s", find(got, "Chrome").NativeHostDir)
	}
	if find(got, "Chrome").WindowsRegistryParent != `Google\Chrome` {
		t.Fatalf("chrome reg = %s", find(got, "Chrome").WindowsRegistryParent)
	}
	if find(got, "Edge").WindowsRegistryParent != `Microsoft\Edge` {
		t.Fatalf("edge reg = %s", find(got, "Edge").WindowsRegistryParent)
	}
}

func TestWindowsFallsBackToAppDataLocal(t *testing.T) {
	got := Candidates(envWith("windows", winHome, "", nil))
	fallback := filepath.Join(winHome, "AppData", "Local")
	if find(got, "Chrome").NativeHostDir != filepath.Join(fallback, "Google", "Chrome", "User Data", "NativeMessagingHosts") {
		t.Fatalf("chrome dir = %s", find(got, "Chrome").NativeHostDir)
	}
}

func TestUnsupportedPlatform(t *testing.T) {
	got := Candidates(envWith("aix", home, "", nil))
	if len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestInstalledRequiresProfileMarker(t *testing.T) {
	base := filepath.Join(home, "Library", "Application Support")
	chromeRoot := filepath.Join(base, "Google", "Chrome")
	chromeNmh := filepath.Join(chromeRoot, "NativeMessagingHosts")

	t.Run("parent dir alone is not enough", func(t *testing.T) {
		env := envWith("darwin", home, "", []string{chromeRoot})
		if n := names(Installed(env)); len(n) != 0 {
			t.Fatalf("got %v", n)
		}
	})
	t.Run("Local State marker", func(t *testing.T) {
		env := envWith("darwin", home, "", []string{chromeRoot, filepath.Join(chromeRoot, "Local State")})
		if n := names(Installed(env)); !equal(n, []string{"Chrome"}) {
			t.Fatalf("got %v", n)
		}
	})
	t.Run("Default profile dir", func(t *testing.T) {
		env := envWith("darwin", home, "", []string{chromeRoot, filepath.Join(chromeRoot, "Default")})
		if n := names(Installed(env)); !equal(n, []string{"Chrome"}) {
			t.Fatalf("got %v", n)
		}
	})
	t.Run("filters browsers without marker", func(t *testing.T) {
		braveRoot := filepath.Join(base, "BraveSoftware", "Brave-Browser")
		env := envWith("darwin", home, "", []string{chromeRoot, filepath.Join(chromeRoot, "Default"), braveRoot})
		if n := names(Installed(env)); !equal(n, []string{"Chrome"}) {
			t.Fatalf("got %v", n)
		}
	})
	t.Run("preserves nativeHostDir", func(t *testing.T) {
		env := envWith("darwin", home, "", []string{chromeRoot, filepath.Join(chromeRoot, "Local State")})
		got := Installed(env)
		if len(got) != 1 || got[0].NativeHostDir != chromeNmh {
			t.Fatalf("got %+v", got)
		}
	})
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
