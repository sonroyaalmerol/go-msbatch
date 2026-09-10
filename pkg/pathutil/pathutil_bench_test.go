package pathutil

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func benchTree(b *testing.B) string {
	b.Helper()
	root := b.TempDir()
	fat := filepath.Join(root, "fat")
	if err := os.MkdirAll(fat, 0o755); err != nil {
		b.Fatal(err)
	}
	for i := range 2000 {
		if err := os.WriteFile(filepath.Join(fat, fmt.Sprintf("f%04d.txt", i)), nil, 0o644); err != nil {
			b.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "deep", "a", "B", "c"), 0o755); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "deep", "a", "B", "c", "target.txt"), nil, 0o644); err != nil {
		b.Fatal(err)
	}
	return root
}

func BenchmarkResolveCaseInsensitiveExact(b *testing.B) {
	root := benchTree(b)
	p := filepath.Join(root, "fat", "f0100.txt")
	for b.Loop() {
		if got := ResolveCaseInsensitive(p); got != p {
			b.Fatalf("got %q", got)
		}
	}
}

func BenchmarkResolveCaseInsensitiveCaseMiss(b *testing.B) {
	root := benchTree(b)
	p := filepath.Join(root, "FAT", "F0100.TXT")
	for b.Loop() {
		_ = ResolveCaseInsensitive(p)
	}
}

func BenchmarkResolveCaseInsensitiveDeepMiss(b *testing.B) {
	root := benchTree(b)
	p := filepath.Join(root, "DEEP", "A", "b", "C", "TARGET.TXT")
	for b.Loop() {
		_ = ResolveCaseInsensitive(p)
	}
}

func BenchmarkMapPath(b *testing.B) {
	root := benchTree(b)
	b.Setenv("MSBATCH_DRIVE_C", root)
	for b.Loop() {
		_ = MapPath(`C:\DEEP\a\B\c\TARGET.TXT`)
	}
}
