//go:build windows

package process

import (
	"os/exec"
	"syscall"
)

func setProcAttr(cmd *exec.Cmd) {}

func killGroup(pid int, sig syscall.Signal) error {
	return nil
}

func isRunning(pid int) bool {
	return false
}
