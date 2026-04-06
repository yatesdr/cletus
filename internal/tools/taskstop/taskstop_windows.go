//go:build windows

package taskstop

import "fmt"

func sendSignal(pid int, force bool) error {
	return fmt.Errorf("sending signals not supported on Windows")
}
