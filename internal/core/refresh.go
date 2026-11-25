package core

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// RefreshAll lê a cota de todas as contas. Para as inativas, troca o login
// temporariamente e sempre devolve o original, mesmo se for interrompido.
// Devolve os avisos de contas que falharam (não é erro fatal).
func (m *Mgr) RefreshAll() (warns []string, err error) {
	unlock, err := m.Lock()
	if err != nil {
		return nil, err
	}
	defer unlock()

	orig := m.ActiveEmail()
	if orig != "" && m.Has(orig) {
		if werr := m.FetchQuota(orig); werr != nil {
			warns = append(warns, fmt.Sprintf("%s: %v", orig, werr))
		}
	}
	if cerr := m.CanSwap(); cerr != nil {
		// sem como trocar: fica só a conta ativa
		return append(warns, "só a conta ativa foi atualizada: "+cerr.Error()), nil
	}
	m.SaveLive()

	restore := func() {
		if orig != "" && m.Has(orig) {
			_ = m.install(orig)
		}
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	done := make(chan struct{})
	defer func() { signal.Stop(sig); close(done) }()
	go func() {
		select {
		case <-sig:
			restore()
			unlock()
			os.Exit(130)
		case <-done:
		}
	}()
	defer restore()

	for _, e := range m.Accounts() {
		if e == orig {
			continue
		}
		if ierr := m.install(e); ierr != nil {
			warns = append(warns, fmt.Sprintf("%s: %v", e, ierr))
			continue
		}
		if werr := m.FetchQuota(e); werr != nil {
			warns = append(warns, fmt.Sprintf("%s: %v", e, werr))
		}
		m.SaveLive() // guarda o token que o agy tenha renovado
	}
	return warns, nil
}
