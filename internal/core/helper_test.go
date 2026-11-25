package core

import (
	"os"
	"os/exec"
	"strconv"
	"testing"
)

// runFake executa o agy de mentira sem argumentos (simula o agy interativo).
func runFake(m *Mgr) error { return exec.Command(m.Agy).Run() }

func itoa(n int) string { return strconv.Itoa(n) }

func writeFile(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeQuota(t *testing.T, m *Mgr, email, tsv string) {
	t.Helper()
	writeFile(t, m.quotaFile(email), tsv)
}
