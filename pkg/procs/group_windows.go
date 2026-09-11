//go:build windows

package procs

import (
	"os/exec"
	"syscall"
)

// ponytail: no group semantics on windows; job objects if descendants ever need killing
func newSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}

func killGroup(c *exec.Cmd) {}
