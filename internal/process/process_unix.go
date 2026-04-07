//go:build !windows

package process

import (
	"os/exec"
	"syscall"
)

func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}
}

func killGroup(pid int, sig syscall.Signal) error {
	return syscall.Kill(-pid, sig)
}

func isRunning(pid int) bool {
	err := syscall.Kill(pid, syscall.Signal(0))
	return err == nil
}
