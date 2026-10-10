package register

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/chiehw/chromium-bookmarks-mcp/internal/types"
)

func installNativeHostBinary() (string, error) {
	dest := types.NativeHostBinaryPath()
	src, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}
	src, err = filepath.EvalSymlinks(src)
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}

	if sameFile(src, dest) {
		return dest, nil
	}
	if upToDate(src, dest) {
		return dest, nil
	}

	if err := copyFile(src, dest); err != nil {
		if _, statErr := os.Stat(dest); statErr == nil {
			// Dest is likely in use (Windows); keep the running copy.
			return dest, nil
		}
		return "", err
	}
	return dest, nil
}

func sameFile(a, b string) bool {
	ai, err1 := os.Stat(a)
	bi, err2 := os.Stat(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return os.SameFile(ai, bi)
}

func upToDate(src, dest string) bool {
	si, err1 := os.Stat(src)
	di, err2 := os.Stat(dest)
	if err1 != nil || err2 != nil {
		return false
	}
	return si.Size() == di.Size() && !si.ModTime().After(di.ModTime())
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dest + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	_ = os.Chmod(dest, 0o755)
	return nil
}
