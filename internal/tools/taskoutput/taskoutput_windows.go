//go:build windows

package taskoutput

func processExists(pid int) bool {
	return false
}
