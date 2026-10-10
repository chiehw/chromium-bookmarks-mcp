package browsers

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type Info struct {
	Name                  string
	NativeHostDir         string
	WindowsRegistryParent string // HKCU\Software\<this>\NativeMessagingHosts; empty off Windows
}

type Env struct {
	GOOS         string
	Home         string
	Exists       func(string) bool
	LocalAppData string
}

func DefaultEnv() Env {
	home, _ := os.UserHomeDir()
	return Env{
		GOOS:         runtime.GOOS,
		Home:         home,
		Exists:       exists,
		LocalAppData: os.Getenv("LOCALAPPDATA"),
	}
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

var profileMarkers = []string{"Local State", "Default"}

func macBrowsers(env Env) []Info {
	base := filepath.Join(env.Home, "Library", "Application Support")
	return []Info{
		{Name: "Chrome", NativeHostDir: filepath.Join(base, "Google", "Chrome", "NativeMessagingHosts")},
		{Name: "Brave", NativeHostDir: filepath.Join(base, "BraveSoftware", "Brave-Browser", "NativeMessagingHosts")},
		{Name: "Edge", NativeHostDir: filepath.Join(base, "Microsoft Edge", "NativeMessagingHosts")},
		{Name: "Arc", NativeHostDir: filepath.Join(base, "Arc", "User Data", "NativeMessagingHosts")},
		{Name: "Chromium", NativeHostDir: filepath.Join(base, "Chromium", "NativeMessagingHosts")},
	}
}

func linuxBrowsers(env Env) []Info {
	home := env.Home
	return []Info{
		{Name: "Chrome", NativeHostDir: filepath.Join(home, ".config", "google-chrome", "NativeMessagingHosts")},
		{Name: "Brave", NativeHostDir: filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser", "NativeMessagingHosts")},
		{Name: "Edge", NativeHostDir: filepath.Join(home, ".config", "microsoft-edge", "NativeMessagingHosts")},
		{Name: "Chromium", NativeHostDir: filepath.Join(home, ".config", "chromium", "NativeMessagingHosts")},
	}
}

func windowsBrowsers(env Env) []Info {
	local := env.LocalAppData
	if local == "" {
		local = filepath.Join(env.Home, "AppData", "Local")
	}
	return []Info{
		{
			Name:                  "Chrome",
			NativeHostDir:         filepath.Join(local, "Google", "Chrome", "User Data", "NativeMessagingHosts"),
			WindowsRegistryParent: `Google\Chrome`,
		},
		{
			Name:                  "Brave",
			NativeHostDir:         filepath.Join(local, "BraveSoftware", "Brave-Browser", "User Data", "NativeMessagingHosts"),
			WindowsRegistryParent: `BraveSoftware\Brave-Browser`,
		},
		{
			Name:                  "Edge",
			NativeHostDir:         filepath.Join(local, "Microsoft", "Edge", "User Data", "NativeMessagingHosts"),
			WindowsRegistryParent: `Microsoft\Edge`,
		},
	}
}

func isInstalled(b Info, env Env) bool {
	profileRoot := filepath.Join(b.NativeHostDir, "..")
	if !env.Exists(profileRoot) {
		return false
	}
	for _, marker := range profileMarkers {
		if env.Exists(filepath.Join(profileRoot, marker)) {
			return true
		}
	}
	return false
}

func Candidates(env Env) []Info {
	switch env.GOOS {
	case "darwin":
		return macBrowsers(env)
	case "linux":
		return linuxBrowsers(env)
	case "windows":
		return windowsBrowsers(env)
	default:
		fmt.Fprintf(os.Stderr, "Unsupported platform: %s. Native host registration not supported.\n", env.GOOS)
		return nil
	}
}

func Installed(env Env) []Info {
	var out []Info
	for _, b := range Candidates(env) {
		if isInstalled(b, env) {
			out = append(out, b)
		}
	}
	return out
}

func InstalledDefault() []Info {
	return Installed(DefaultEnv())
}
