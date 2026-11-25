package core

import "os/exec"

// runFake executa o agy de mentira sem argumentos (simula o agy interativo).
func runFake(m *Mgr) error { return exec.Command(m.Agy).Run() }
