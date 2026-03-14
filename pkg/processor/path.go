package processor

import (
	"path/filepath"
	"runtime"
	"strings"
)

// MapPath converts a Windows-style path to a platform-appropriate path.
// On non-Windows platforms, it maps backslashes to forward slashes and
// handles drive letters (e.g., C:\foo -> /mnt/c/foo).
func MapPath(path string) string {
	if runtime.GOOS == "windows" {
		return filepath.Clean(path)
	}

	// Replace backslashes with forward slashes
	p := strings.ReplaceAll(path, "\\", "/")

	// Handle drive letters: C:/foo -> /mnt/c/foo
	if len(p) >= 2 && p[1] == ':' && ((p[0] >= 'a' && p[0] <= 'z') || (p[0] >= 'A' && p[0] <= 'Z')) {
		drive := strings.ToLower(string(p[0]))
		p = "/mnt/" + drive + p[2:]
	}

	return filepath.Clean(p)
}
