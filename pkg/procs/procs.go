// Package procs runs child commands in their own process group so context cancellation reaches descendants.
package procs

import (
	"context"
	"os/exec"
)

// Run starts c in a new process group, waits for it, and kills the whole
// group when ctx is cancelled. Killing only the direct child leaves
// descendants holding inherited pipes, which blocks Wait.
func Run(ctx context.Context, c *exec.Cmd) error {
	if c.SysProcAttr == nil {
		c.SysProcAttr = newSysProcAttr()
	}
	if err := c.Start(); err != nil {
		return err
	}
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			killGroup(c)
		case <-done:
		}
	}()
	err := c.Wait()
	close(done)
	return err
}
