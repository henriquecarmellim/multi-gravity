//go:build !linux && !windows

package core

import "os/exec"

// AgyRunning procura um processo chamado "agy" (macOS, BSD).
func AgyRunning() bool { return exec.Command("pgrep", "-x", "agy").Run() == nil }
