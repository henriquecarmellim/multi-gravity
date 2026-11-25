//go:build windows

package core

import (
	"os/exec"
	"strings"
)

// AgyRunning procura um processo agy.exe.
func AgyRunning() bool {
	out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq agy.exe", "/NH").Output()
	return err == nil && strings.Contains(strings.ToLower(string(out)), "agy.exe")
}
