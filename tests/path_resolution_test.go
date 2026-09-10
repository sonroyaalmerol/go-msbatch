package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetPathDrivesResolution(t *testing.T) {
	dir := isolate(t)
	if err := os.WriteFile(filepath.Join(dir, "hello-tool"), []byte("#!/bin/sh\necho from-path-dir\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := runScript(t, "@echo off\nset PATH="+dir+";%PATH%\nhello-tool\n")
	if got.stdout != "from-path-dir\n" {
		t.Errorf("stdout = %q, want %q", got.stdout, "from-path-dir\n")
	}

	got = runScript(t, "@echo off\nset PATH="+dir+";%PATH%\nwhere hello-tool\n")
	if !strings.HasSuffix(got.stdout, "hello-tool\n") {
		t.Errorf("where output = %q, want path ending in hello-tool", got.stdout)
	}

	t.Setenv("MSBATCH_DRIVE_X", dir)
	got = runScript(t, "@echo off\nset PATH=X:\\;%PATH%\nhello-tool\n")
	if got.stdout != "from-path-dir\n" {
		t.Errorf("windows-style stdout = %q, want %q", got.stdout, "from-path-dir\n")
	}
}
