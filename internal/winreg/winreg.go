package winreg

import (
	"runtime"
	"strings"
)

func NormalizePath(p string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(p), `/\`)
	if runtime.GOOS == "windows" {
		return strings.ToLower(trimmed)
	}
	return trimmed
}

func RegistryKey(windowsRegistryParent, hostName string) string {
	return `HKCU\Software\` + windowsRegistryParent + `\NativeMessagingHosts\` + hostName
}
