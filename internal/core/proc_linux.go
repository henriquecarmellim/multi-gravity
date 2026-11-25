//go:build linux

package core

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// AgyRunning procura um processo chamado "agy".
func AgyRunning() bool {
	ents, _ := os.ReadDir("/proc")
	for _, e := range ents {
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		b, err := os.ReadFile(filepath.Join("/proc", e.Name(), "comm"))
		if err == nil && strings.TrimSpace(string(b)) == "agy" {
			return true
		}
	}
	return false
}
