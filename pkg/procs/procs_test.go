package procs

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

// A backgrounded descendant inheriting stdout blocks Wait until it exits;
// group kill must unblock it as soon as the context is cancelled.
func TestRunKillsDescendantsOnCancel(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns long sleeps")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.Command("sh", "-c", "echo start; sleep 30 & sleep 30")
	out := &lockingBuffer{}
	cmd.Stdout = out
	cmd.Stderr = out
	done := make(chan error, 1)
	go func() { done <- Run(ctx, cmd) }()
	time.Sleep(300 * time.Millisecond)
	start := time.Now()
	cancel()
	select {
	case <-done:
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Fatalf("Run took %v to unblock after cancel", elapsed)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run blocked on descendant holding the pipe")
	}
}

type lockingBuffer struct {
	data []byte
}

func (b *lockingBuffer) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	return len(p), nil
}
