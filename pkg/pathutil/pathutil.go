package pathutil

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func StripQuotes(s string) string {
	if len(s) >= 2 {
		q := s[0]
		if (q == '"' || q == '\'' || q == '`') && s[len(s)-1] == q {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func DriveMount(letter byte) string {
	lower := strings.ToLower(string(letter))
	upper := strings.ToUpper(lower)

	if v := os.Getenv("MSBATCH_DRIVE_" + upper); v != "" {
		return strings.TrimRight(v, "/")
	}

	if root := os.Getenv("MSBATCH_DRIVE_ROOT"); root != "" {
		if !strings.HasSuffix(root, "/") {
			root += "/"
		}
		return strings.TrimRight(root+lower, "/")
	}

	if lower == "z" {
		return ""
	}

	return "/mnt/" + lower
}

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

func uncMount(server, share string) string {
	sk := uncEnvKey(server)
	hk := uncEnvKey(share)

	if v := os.Getenv("MSBATCH_UNC_" + sk + "_" + hk); v != "" {
		return strings.TrimRight(v, "/")
	}

	if v := os.Getenv("MSBATCH_UNC_" + sk); v != "" {
		return strings.TrimRight(v, "/") + "/" + strings.ToLower(share)
	}

	if root := os.Getenv("MSBATCH_UNC_ROOT"); root != "" {
		return strings.TrimRight(root, "/") + "/" + strings.ToLower(server) + "/" + strings.ToLower(share)
	}

	return ""
}

func ResolveCaseInsensitive(path string) string {
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
			return path
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
			return path
		}
	}

	if strings.HasPrefix(path, "./") && !strings.HasPrefix(currentPath, "./") {
		return "./" + currentPath
	} else if !strings.HasPrefix(path, "./") && strings.HasPrefix(currentPath, ".") && currentPath != "." {
		return strings.TrimPrefix(currentPath, "./")
	}

	return currentPath
}

func MapPath(path string) string {
	if runtime.GOOS == "windows" {
		return filepath.Clean(path)
	}

	path = StripQuotes(path)

	p := strings.ReplaceAll(path, "\\", "/")

	if strings.HasPrefix(p, "//") {
		parts := strings.SplitN(p[2:], "/", 3)
		if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
			server, share := parts[0], parts[1]
			mount := uncMount(server, share)
			if mount == "" {
				return ResolveCaseInsensitive(filepath.Clean(p))
			}
			rest := ""
			if len(parts) == 3 {
				rest = "/" + parts[2]
			}
			return ResolveCaseInsensitive(filepath.Clean(mount + rest))
		}
	}

	if len(p) >= 2 && p[1] == ':' && ((p[0] >= 'a' && p[0] <= 'z') || (p[0] >= 'A' && p[0] <= 'Z')) {
		mount := DriveMount(p[0])
		p = mount + p[2:]
	}

	return ResolveCaseInsensitive(filepath.Clean(p))
}

func IsPathLike(s string) bool {
	return strings.HasPrefix(s, "/") ||
		strings.HasPrefix(s, "./") ||
		strings.HasPrefix(s, "../") ||
		strings.Contains(s, "/")
}

func IsWindowsPathLike(s string) bool {
	return strings.Contains(s, "\\") || (len(s) >= 2 && s[1] == ':')
}

func MapArg(arg string) string {
	if IsWindowsPathLike(arg) {
		return MapPath(arg)
	}

	if runtime.GOOS != "windows" && IsPathLike(arg) {
		return ResolveCaseInsensitive(arg)
	}

	return arg
}

func MapArgForWine(arg string) string {
	if IsWindowsPathLike(arg) {
		unixPath := MapPath(arg)
		resolved := ResolveCaseInsensitive(unixPath)
		return UnixToWinePath(resolved)
	}

	if IsPathLike(arg) {
		return ResolveCaseInsensitive(arg)
	}

	return arg
}

func UnixToWinePath(unixPath string) string {
	if unixPath == "" || unixPath == "/" {
		return "Z:\\"
	}

	if !strings.HasPrefix(unixPath, "/") {
		return unixPath
	}

	for drive := 'C'; drive <= 'Z'; drive++ {
		mount := DriveMount(byte(drive))
		if mount == "" {
			continue
		}
		if strings.HasPrefix(unixPath, mount+"/") || unixPath == mount {
			rel := strings.TrimPrefix(unixPath, mount)
			return string(drive) + ":" + strings.ReplaceAll(rel, "/", "\\")
		}
	}

	rel := strings.TrimPrefix(unixPath, "/")
	return "Z:\\" + strings.ReplaceAll(rel, "/", "\\")
}

func HasWildcard(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[")
}

func MatchCaseInsensitive(pattern, name string) bool {
	if runtime.GOOS == "windows" {
		matched, _ := filepath.Match(pattern, name)
		return matched
	}

	matched, _ := filepath.Match(strings.ToLower(pattern), strings.ToLower(name))
	return matched
}

func GlobCaseInsensitive(pattern string) ([]string, error) {
	if runtime.GOOS == "windows" {
		return filepath.Glob(pattern)
	}

	if !HasWildcard(pattern) {
		return filepath.Glob(pattern)
	}

	dir := filepath.Dir(pattern)
	base := filepath.Base(pattern)

	if dir == "" || dir == "." {
		dir = "."
	} else {
		dir = ResolveCaseInsensitive(dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}

	patternLower := strings.ToLower(base)
	var result []string

	for _, entry := range entries {
		if matched, _ := filepath.Match(patternLower, strings.ToLower(entry.Name())); matched {
			result = append(result, filepath.Join(dir, entry.Name()))
		}
	}

	return result, nil
}
