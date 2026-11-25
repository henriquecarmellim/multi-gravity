//go:build !windows

package main

import (
	"os"
	"syscall"
)

// launchAgy troca o processo do mgy pelo agy (sem processo extra no meio).
func launchAgy(bin string, args []string) error {
	return syscall.Exec(bin, append([]string{"agy"}, args...), os.Environ())
}
