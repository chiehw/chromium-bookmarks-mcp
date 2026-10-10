//go:build windows

package winreg

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func splitHKCU(keyPath string) (string, error) {
	normalized := strings.ReplaceAll(keyPath, `/`, `\`)
	const prefix = `HKCU\`
	if !strings.HasPrefix(strings.ToUpper(normalized), "HKCU\\") {
		return "", fmt.Errorf("registry key must start with HKCU\\: %s", keyPath)
	}
	return normalized[len(prefix):], nil
}

func Add(keyPath, value string) error {
	sub, err := splitHKCU(keyPath)
	if err != nil {
		return err
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, sub, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("reg add %s: %w", keyPath, err)
	}
	defer k.Close()
	if err := k.SetStringValue("", value); err != nil {
		return fmt.Errorf("reg add %s: %w", keyPath, err)
	}
	return nil
}

func Delete(keyPath string) error {
	sub, err := splitHKCU(keyPath)
	if err != nil {
		return err
	}
	err = registry.DeleteKey(registry.CURRENT_USER, sub)
	if err == nil || errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("reg delete %s: %w", keyPath, err)
}

func Query(keyPath string) (string, error) {
	sub, err := splitHKCU(keyPath)
	if err != nil {
		return "", err
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, sub, registry.QUERY_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("reg query %s: %w", keyPath, err)
	}
	defer k.Close()
	val, _, err := k.GetStringValue("")
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("reg query %s: %w", keyPath, err)
	}
	return val, nil
}
