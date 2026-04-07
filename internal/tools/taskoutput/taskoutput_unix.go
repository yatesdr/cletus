//go:build !windows

package taskoutput

import "golang.org/x/sys/unix"

func processExists(pid int) bool {
	return unix.Kill(pid, 0) == nil
}
