//go:build windows

package core

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

const exeSuffix = ".exe"

// dataHome: %LOCALAPPDATA% (dados que não devem ir para outros PCs).
func dataHome(home string) string {
	if d := os.Getenv("LOCALAPPDATA"); d != "" {
		return d
	}
	return filepath.Join(home, "AppData", "Local")
}

func lockFile(f *os.File) error {
	return windows.LockFileEx(windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, new(windows.Overlapped))
}

func unlockFile(f *os.File) {
	_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, new(windows.Overlapped))
}
