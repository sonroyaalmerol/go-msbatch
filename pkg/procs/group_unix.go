//go:build !windows

package procs

import (
	"os/exec"
	"syscall"
)

func newSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

func killGroup(c *exec.Cmd) {
	if c.Process == nil {
		return
	}
	syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
}
