package executor

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// VersionString returns the string printed by the VER command and the
// interactive session banner.
//
// It can be overridden by setting MSBATCH_VERSION in the host environment:
//
//	MSBATCH_VERSION="MyApp Runner [Version 2.1.0]"
//
// When unset, the default is derived from the host OS at runtime:
//   - Non-Windows: "uname -s" + "uname -r" → e.g. "Linux [Version 6.19.5-3-cachyos]"
//   - Windows:     output of "cmd /c ver"   → e.g. "Microsoft Windows [Version 10.0.19045]"
//   - Fallback (if exec fails): runtime.GOOS
func VersionString() string {
	if v := os.Getenv("MSBATCH_VERSION"); v != "" {
		return v
	}
	return hostVersionString()
}

func hostVersionString() string {
	if runtime.GOOS == "windows" {
		out, err := exec.Command("cmd", "/c", "ver").Output()
		if err == nil {
			if s := strings.TrimSpace(string(out)); s != "" {
				return s
			}
		}
		return "Windows"
	}

	name, errN := exec.Command("uname", "-s").Output()
	rel, errR := exec.Command("uname", "-r").Output()
	n := strings.TrimSpace(string(name))
	r := strings.TrimSpace(string(rel))
	if errN == nil && errR == nil && n != "" && r != "" {
		return n + " [Version " + r + "]"
	}
	return runtime.GOOS
}
