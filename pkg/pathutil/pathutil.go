package pathutil

import (
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
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
		v = strings.TrimRight(v, "/")
		if v == "" {
			return "/"
		}
		return v
	}

	if prefix := os.Getenv("MSBATCH_PREFIX"); prefix != "" {
		prefix = strings.TrimRight(prefix, "/")
		if lower == "z" {
			return ""
		}
		return prefix + "/drive_" + lower
	}

	if lower == "z" {
		return ""
	}

	return "drive_" + lower
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

type dirCacheEntry struct {
	modTime time.Time
	names   []string
}

var (
	dirCacheMu sync.Mutex
	dirCache   = map[string]dirCacheEntry{}
)

const dirCacheMax = 4096

var dirCacheOff = sync.OnceValue(func() bool {
	return os.Getenv("MSBATCH_NO_FS_CACHE") != ""
})

func readDirCached(dir string) ([]string, bool) {
	if dirCacheOff() {
		return dirEntryNames(dir), false
	}

	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		return nil, false
	}
	modTime := fi.ModTime()

	dirCacheMu.Lock()
	e, ok := dirCache[dir]
	dirCacheMu.Unlock()
	if ok && e.modTime.Equal(modTime) {
		return e.names, time.Since(modTime) <= time.Second
	}

	names := dirEntryNames(dir)
	if names == nil {
		return nil, false
	}

	dirCacheMu.Lock()
	if len(dirCache) >= dirCacheMax {
		clear(dirCache)
	}
	dirCache[dir] = dirCacheEntry{modTime: modTime, names: names}
	dirCacheMu.Unlock()
	return names, false
}

func dirEntryNames(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	names := make([]string, len(entries))
	for i, ent := range entries {
		names[i] = ent.Name()
	}
	return names
}

func scanEntryNames(names []string, part string) string {
	for _, name := range names {
		if strings.EqualFold(name, part) {
			return name
		}
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

	for i, part := range parts {
		if part == "" || part == "." || part == ".." {
			currentPath = filepath.Join(currentPath, part)
			continue
		}

		names, stale := readDirCached(currentPath)
		if names == nil {
			if strings.ContainsAny(part, "*?[") {
				return currentPath + "/" + part
			}
			return path
		}

		matchName := scanEntryNames(names, part)
		if stale {
			if matchName != "" {
				if _, err := os.Stat(filepath.Join(currentPath, matchName)); err != nil {
					matchName = ""
				}
			}
			if matchName == "" {
				if fresh := dirEntryNames(currentPath); fresh != nil {
					names = fresh
					matchName = scanEntryNames(names, part)
				}
			}
		}

		if matchName != "" {
			currentPath = filepath.Join(currentPath, matchName)
			continue
		}

		if strings.ContainsAny(part, "*?[") {
			if filepath.IsAbs(path) && currentPath == "/" {
				return "/" + part
			}
			return currentPath + "/" + strings.Join(parts[i:], "/")
		}
		return path
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
		rest := p[2:]
		switch {
		case rest == "":
			if cur := DriveDir(p[0]); cur != "" {
				p = cur
			} else {
				p = mount
			}
		case rest[0] != '/':
			base := DriveDir(p[0])
			if base == "" {
				base = mount
			}
			p = base + "/" + rest
		default:
			p = mount + rest
		}
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

	toDrive := func(drive byte, mount, p string) string {
		rel := strings.TrimPrefix(p, mount)
		if rel == "" {
			return string(drive) + ":\\"
		}
		return string(drive) + ":\\" + strings.TrimPrefix(strings.ReplaceAll(rel, "/", "\\"), "\\")
	}

	if !strings.HasPrefix(unixPath, "/") {
		for drive := 'C'; drive <= 'Z'; drive++ {
			mount := DriveMount(byte(drive))
			if mount == "" {
				continue
			}
			if mount == "/" || strings.HasPrefix(unixPath, mount+"/") || unixPath == mount {
				return toDrive(byte(drive), mount, unixPath)
			}
		}
		return unixPath
	}

	for drive := 'C'; drive <= 'Z'; drive++ {
		mount := DriveMount(byte(drive))
		if mount == "" {
			continue
		}
		if mount == "/" || strings.HasPrefix(unixPath, mount+"/") || unixPath == mount {
			return toDrive(byte(drive), mount, unixPath)
		}
	}

	rel := strings.TrimPrefix(unixPath, "/")
	return "Z:\\" + strings.ReplaceAll(rel, "/", "\\")
}

func ToWindowsPath(unixPath string) string {
	if runtime.GOOS == "windows" {
		return unixPath
	}
	return UnixToWinePath(unixPath)
}

var ErrNotFound = os.ErrNotExist

// LookPathIn resolves name against a caller-supplied PATH list, MapPath'ing
// Windows-style entries so batch SET PATH values resolve on Unix hosts.
func LookPathIn(pathList, name string) (string, error) {
	if strings.ContainsAny(name, `\/`) {
		if isExecutableFile(MapPath(name)) {
			return MapPath(name), nil
		}
		return "", ErrNotFound
	}
	for _, dir := range SplitPathList(pathList) {
		if dir == "" {
			dir = "."
		}
		candidate := filepath.Join(MapPath(dir), name)
		if isExecutableFile(candidate) {
			return candidate, nil
		}
	}
	return "", ErrNotFound
}

// SplitPathList splits a PATH-style list on ';' and ':', keeping drive-letter
// colons (C:\) intact. Batch SET PATH mixes both separator styles on Unix.
func SplitPathList(list string) []string {
	var parts []string
	var cur []byte
	for i := 0; i < len(list); i++ {
		c := list[i]
		if c == ';' || (c == ':' && !isDriveColon(list, i)) {
			parts = append(parts, string(cur))
			cur = cur[:0]
			continue
		}
		cur = append(cur, c)
	}
	return append(parts, string(cur))
}

func isDriveColon(list string, i int) bool {
	return i == 1 && isAlphaByte(list[0]) && i+1 < len(list) && (list[i+1] == '\\' || list[i+1] == '/')
}

func isAlphaByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode().Perm()&0111 != 0
}

var (
	driveDirsMu sync.RWMutex
	driveDirs   = map[byte]string{}
)

func upperDrive(letter byte) byte {
	if letter >= 'a' && letter <= 'z' {
		return letter - ('a' - 'A')
	}
	return letter
}

// SetDriveDir records dir as the current directory of a drive. cmd.exe keeps one current directory per drive, which is what "X:rel" and a bare "X:" resolve against.
func SetDriveDir(letter byte, dir string) {
	driveDirsMu.Lock()
	driveDirs[upperDrive(letter)] = dir
	driveDirsMu.Unlock()
}

// DriveDir returns the recorded current directory of a drive, or "" if the drive has not been visited yet.
func DriveDir(letter byte) string {
	driveDirsMu.RLock()
	defer driveDirsMu.RUnlock()
	return driveDirs[upperDrive(letter)]
}

func SnapshotDriveDirs() map[byte]string {
	driveDirsMu.RLock()
	defer driveDirsMu.RUnlock()
	return maps.Clone(driveDirs)
}

func RestoreDriveDirs(dirs map[byte]string) {
	driveDirsMu.Lock()
	driveDirs = maps.Clone(dirs)
	driveDirsMu.Unlock()
}

// Chdir changes the process directory and records its drive state. pi:keep
func Chdir(dir string) error {
	if err := os.Chdir(dir); err != nil {
		return err
	}
	abs, err := os.Getwd()
	if err != nil {
		abs = dir
	}
	if w := ToWindowsPath(abs); len(w) >= 2 && w[1] == ':' {
		SetDriveDir(w[0], abs)
	}
	return nil
}

func IsRooted(p string) bool {
	return strings.HasPrefix(p, "/") || strings.HasPrefix(p, `\`) ||
		(len(p) >= 2 && p[1] == ':')
}

// SplitWindows splits p into drive, directory (with trailing separator) and base name, treating both separators like cmd.exe rather than only '/' as path/filepath does on Unix.
func SplitWindows(p string) (drive, dir, base string) {
	if len(p) >= 2 && p[1] == ':' {
		drive, p = p[:2], p[2:]
	}
	if i := strings.LastIndexAny(p, `\/`); i >= 0 {
		return drive, p[:i+1], p[i+1:]
	}
	return drive, "", p
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

	names := dirEntryNames(dir)
	if names == nil {
		return nil, nil
	}

	patternLower := strings.ToLower(base)
	var result []string

	for _, name := range names {
		if matched, _ := filepath.Match(patternLower, strings.ToLower(name)); matched {
			result = append(result, filepath.Join(dir, name))
		}
	}

	return result, nil
}

func IsWindowsDevice(name string) bool {
	base := filepath.Base(name)
	if i := strings.Index(base, "."); i >= 0 {
		base = base[:i]
	}
	base = strings.ToUpper(base)
	switch base {
	case "NUL", "CON", "PRN", "AUX":
		return true
	}
	if len(base) == 4 && strings.HasPrefix(base, "COM") {
		c := base[3]
		return c >= '1' && c <= '9'
	}
	if len(base) == 4 && strings.HasPrefix(base, "LPT") {
		c := base[3]
		return c >= '1' && c <= '9'
	}
	return false
}
