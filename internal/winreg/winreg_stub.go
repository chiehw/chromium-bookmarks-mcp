//go:build !windows

package winreg

func Add(keyPath, value string) error { return nil }

func Delete(keyPath string) error { return nil }

func Query(keyPath string) (string, error) { return "", nil }
