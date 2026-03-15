// Package tools provides native Go implementations of commonly-used external
// commands. These work cross-platform without requiring the host tool to be
// installed.
package tools

import (
	"io"
	"os"
)

// isDirEmpty reports whether the directory at path contains no entries.
func isDirEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

// copyFile copies a single file from src to dst, preserving the source
// modification time so that subsequent robocopy runs can detect unchanged files.
func copyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	out.Close()

	return os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime())
}
