package process

import (
	"os/exec"
	"syscall"
)

// Group manages process groups for clean termination.
type Group struct {
	cmd *exec.Cmd
}

// NewGroup creates a new process group.
func NewGroup(cmd *exec.Cmd) *Group {
	setProcAttr(cmd)
	return &Group{cmd: cmd}
}

// Kill terminates the entire process group.
func (g *Group) Kill() error {
	if g.cmd.Process == nil {
		return nil
	}
	return killGroup(g.cmd.Process.Pid, syscall.SIGKILL)
}

// Signal sends a signal to the process group.
func (g *Group) Signal(sig syscall.Signal) error {
	if g.cmd.Process == nil {
		return nil
	}
	return killGroup(g.cmd.Process.Pid, sig)
}

// PID returns the process ID.
func (g *Group) PID() int {
	if g.cmd.Process == nil {
		return 0
	}
	return g.cmd.Process.Pid
}

// IsRunning checks if the process is still running.
func (g *Group) IsRunning() bool {
	if g.cmd.Process == nil {
		return false
	}
	return isRunning(g.cmd.Process.Pid)
}

// NewCommand creates a command with a process group.
func NewCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	NewGroup(cmd)
	return cmd
}
