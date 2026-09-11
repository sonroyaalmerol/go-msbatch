package msbatch_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	msbatch "github.com/sonroyaalmerol/go-msbatch"
)

func TestRunFileWithCustomCommand(t *testing.T) {
	dir := t.TempDir()
	batch := filepath.Join(dir, "job.bat")
	if err := os.WriteFile(batch, []byte("@echo off\r\nrecord one two\r\nexit /b 7\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	interpreter := msbatch.New()
	interpreter.Handle("record", func(ctx context.Context, session *msbatch.Session, command msbatch.Command) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		fmt.Fprintln(session.Stdout(), strings.Join(command.Arguments, ","))
		session.SetEnv("RECORDED", "yes")
		return nil
	})

	result, err := interpreter.RunFile(context.Background(), "job.bat", []string{"arg"}, msbatch.Options{
		Dir:         dir,
		Environment: map[string]string{"PATH": os.Getenv("PATH")},
		Stdout:      &stdout,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 7 {
		t.Fatalf("exit code = %d, want 7", result.ExitCode)
	}
	if got := result.Environment["RECORDED"]; got != "yes" {
		t.Fatalf("RECORDED = %q, want yes", got)
	}
	if got := stdout.String(); got != "one,two\r\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestRunFileCancelsInsideCalledBatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses the POSIX sleep command")
	}
	dir := t.TempDir()
	inner := filepath.Join(dir, "inner.bat")
	if err := os.WriteFile(inner, []byte("@echo off\r\nsleep 30\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	batch := filepath.Join(dir, "job.bat")
	if err := os.WriteFile(batch, []byte("@echo off\r\ncall inner.bat\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := msbatch.New().RunFile(ctx, batch, nil, msbatch.Options{
		Dir:         dir,
		Environment: map[string]string{"PATH": os.Getenv("PATH")},
		Stdout:      &bytes.Buffer{},
		Stderr:      &bytes.Buffer{},
	})
	if !strings.Contains(fmt.Sprint(err), context.DeadlineExceeded.Error()) {
		t.Fatalf("error = %v, want deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("cancellation took %s", elapsed)
	}
}

func TestRunFileCancelsExternalCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses the POSIX sleep command")
	}
	dir := t.TempDir()
	batch := filepath.Join(dir, "job.bat")
	if err := os.WriteFile(batch, []byte("@echo off\r\nsleep 10\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := msbatch.New().RunFile(ctx, batch, nil, msbatch.Options{
		Environment: map[string]string{"PATH": os.Getenv("PATH")},
		Stdout:      &bytes.Buffer{},
		Stderr:      &bytes.Buffer{},
	})
	if !strings.Contains(fmt.Sprint(err), context.DeadlineExceeded.Error()) {
		t.Fatalf("error = %v, want deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("cancellation took %s", elapsed)
	}
}
