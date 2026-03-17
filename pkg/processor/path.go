package processor

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// driveMount returns the Unix mount point for a single drive letter.
//
// Lookup order (first non-empty value wins):
//  1. MSBATCH_DRIVE_<LETTER>  — per-drive override, e.g. MSBATCH_DRIVE_C=/mnt/c
//  2. MSBATCH_DRIVE_ROOT      — common prefix, e.g. MSBATCH_DRIVE_ROOT=/drives/
//     → resolves to <prefix><lowercase-letter>
//  3. Built-in default /mnt/<lowercase-letter>  (WSL2 convention)
func driveMount(letter byte) string {
	lower := strings.ToLower(string(letter))
	upper := strings.ToUpper(lower)

	// 1. Per-drive override.
	if v := os.Getenv("MSBATCH_DRIVE_" + upper); v != "" {
		return strings.TrimRight(v, "/")
	}

	// 2. Common prefix.
	if root := os.Getenv("MSBATCH_DRIVE_ROOT"); root != "" {
		if !strings.HasSuffix(root, "/") {
			root += "/"
		}
		return strings.TrimRight(root+lower, "/")
	}

	// 3. Default WSL2-style mount.
	return "/mnt/" + lower
}

// uncEnvKey converts a server or share name into an environment-variable-safe
// identifier: uppercased, with runs of non-alphanumeric characters replaced
// by a single underscore.
func uncEnvKey(s string) string {
	s = strings.ToUpper(s)
	var b strings.Builder
	prevUnderscore := false
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevUnderscore = false
		} else if !prevUnderscore {
			b.WriteByte('_')
			prevUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

// uncMount returns the Unix path for a UNC server+share pair.
//
// Lookup order (first non-empty value wins):
//  1. MSBATCH_UNC_<SERVER>_<SHARE>  — exact mount for this server/share pair.
//     \\myserver\docs  →  MSBATCH_UNC_MYSERVER_DOCS=/home/user/docs
//  2. MSBATCH_UNC_<SERVER>           — root for all shares on this server;
//     share name is appended as a subdirectory.
//     \\myserver\docs  →  MSBATCH_UNC_MYSERVER=/mnt/myserver  →  /mnt/myserver/docs
//  3. MSBATCH_UNC_ROOT               — root for all UNC paths;
//     server and share are appended as subdirectories.
//     \\myserver\docs  →  MSBATCH_UNC_ROOT=/mnt/unc  →  /mnt/unc/myserver/docs
//  4. No default — returns "" to signal "unmapped".
func uncMount(server, share string) string {
	sk := uncEnvKey(server)
	hk := uncEnvKey(share)

	// 1. Per server+share.
	if v := os.Getenv("MSBATCH_UNC_" + sk + "_" + hk); v != "" {
		return strings.TrimRight(v, "/")
	}

	// 2. Per server.
	if v := os.Getenv("MSBATCH_UNC_" + sk); v != "" {
		return strings.TrimRight(v, "/") + "/" + strings.ToLower(share)
	}

	// 3. Common UNC root.
	if root := os.Getenv("MSBATCH_UNC_ROOT"); root != "" {
		return strings.TrimRight(root, "/") + "/" + strings.ToLower(server) + "/" + strings.ToLower(share)
	}

	return ""
}

func resolveCaseInsensitive(path string) string {
	if _, err := os.Stat(path); err == nil {
		return path
	}

	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return path
	}

	var currentPath string
	if filepath.IsAbs(path) {
		currentPath = "/"
		parts = parts[1:]
	} else {
		currentPath = "."
	}

	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			currentPath = filepath.Join(currentPath, part)
			continue
		}

		entries, err := os.ReadDir(currentPath)
		if err != nil {
			return path // fallback
		}

		matched := false
		for _, entry := range entries {
			if strings.EqualFold(entry.Name(), part) {
				currentPath = filepath.Join(currentPath, entry.Name())
				matched = true
				break
			}
		}

		if !matched {
			return path // fallback
		}
	}

	if strings.HasPrefix(path, "./") && !strings.HasPrefix(currentPath, "./") {
		return "./" + currentPath
	} else if !strings.HasPrefix(path, "./") && strings.HasPrefix(currentPath, ".") && currentPath != "." {
		return strings.TrimPrefix(currentPath, "./")
	}

	return currentPath
}

// MapPath converts a Windows-style path to a platform-appropriate path.
// On non-Windows platforms, it maps backslashes to forward slashes and
// resolves drive letters and UNC paths using the MSBATCH_* environment
// variables described in driveMount and uncMount.
func MapPath(path string) string {
	if runtime.GOOS == "windows" {
		return filepath.Clean(path)
	}

	// Strip outer quotes before processing the path.
	if len(path) >= 2 && (path[0] == '"' || path[0] == '\'') && path[len(path)-1] == path[0] {
		path = path[1 : len(path)-1]
	}

	// Replace backslashes with forward slashes.
	p := strings.ReplaceAll(path, "\\", "/")

	// UNC path: //server/share[/rest]
	if strings.HasPrefix(p, "//") {
		// Strip leading slashes and split into components.
		parts := strings.SplitN(p[2:], "/", 3)
		if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
			server, share := parts[0], parts[1]
			mount := uncMount(server, share)
			if mount == "" {
				// No mapping configured — return the path unchanged so callers
				// can decide how to handle it.
				return resolveCaseInsensitive(filepath.Clean(p))
			}
			rest := ""
			if len(parts) == 3 {
				rest = "/" + parts[2]
			}
			return resolveCaseInsensitive(filepath.Clean(mount + rest))
		}
	}

	// Drive letter: C:/foo  →  <mount>/foo
	if len(p) >= 2 && p[1] == ':' && ((p[0] >= 'a' && p[0] <= 'z') || (p[0] >= 'A' && p[0] <= 'Z')) {
		mount := driveMount(p[0])
		p = mount + p[2:]
	}

	return resolveCaseInsensitive(filepath.Clean(p))
}
