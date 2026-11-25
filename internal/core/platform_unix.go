//go:build !windows

package core

import (
	"os"
	"path/filepath"
	"syscall"
)

const exeSuffix = ""

// dataHome: onde ficam os dados do usuário (XDG).
func dataHome(home string) string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return d
	}
	return filepath.Join(home, ".local", "share")
}

func lockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

func unlockFile(f *os.File) { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }
