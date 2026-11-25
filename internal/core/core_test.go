package core

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func tokenJSON(email string) string {
	return `{"token":{"access_token":"a-` + email + `","refresh_token":"r","token_type":"Bearer"},"auth_method":"consumer","id_token_email":"` + email + `"}`
}

// fake agy: "-p /quota" imprime uma cota que depende da conta ativa;
// sem argumentos e sem login, "loga" como novo@teste.com (simula o login no navegador).
const fakeAgy = `#!/bin/sh
LIVE="$MGY_LIVE"
if [ "$1" = "-p" ]; then
  [ -f "$LIVE" ] || exit 1
  if grep -q 'ruim@teste.com' "$LIVE"; then exit 1; fi
  printf 'Gemini Models\tWeekly Limit Remaining\t42%%\t2026-12-01T00:00:00Z\n'
  printf 'Gemini Models\tFive Hour Limit Remaining\t100%%\t2026-11-25T18:00:00Z\n'
  printf 'Claude and GPT models\tWeekly Limit Remaining\t0%%\t2026-11-25T16:00:00Z\n'
  printf 'Claude and GPT models\tFive Hour Limit Remaining\tdisabled\t\n'
  exit 0
fi
[ -f "$LIVE" ] || printf '%s' '{"token":{"access_token":"x","refresh_token":"y"},"id_token_email":"novo@teste.com"}' > "$LIVE"
`

func setup(t *testing.T) *Mgr {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("o agy de mentira dos testes é um script sh")
	}
	dir := t.TempDir()
	agy := filepath.Join(dir, "agy")
	if err := os.WriteFile(agy, []byte(fakeAgy), 0o755); err != nil {
		t.Fatal(err)
	}
	live := filepath.Join(dir, "live", "antigravity-oauth-token")
	os.MkdirAll(filepath.Dir(live), 0o700)
	t.Setenv("MGY_LIVE", live)
	agyRunning = func() bool { return false }
	t.Cleanup(func() { agyRunning = AgyRunning })
	m := &Mgr{Live: live, Store: filepath.Join(dir, "store"), Agy: agy}
	if err := m.Init(); err != nil {
		t.Fatal(err)
	}
	return m
}

func login(t *testing.T, m *Mgr, email string) {
	t.Helper()
	if err := os.WriteFile(m.Live, []byte(tokenJSON(email)), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestParseQuota(t *testing.T) {
	ls := ParseQuota("Gemini Models\tWeekly Limit Remaining\t6%\t2026-11-30T20:10:30Z\nClaude and GPT models\tFive Hour Limit Remaining\tdisabled\t\nlixo sem tabs\n")
	if len(ls) != 2 {
		t.Fatalf("esperava 2 limites, veio %d", len(ls))
	}
	if !ls[0].HasPct || ls[0].Pct != 6 || ls[0].Reset.IsZero() || ls[0].Column() != 1 {
		t.Errorf("limite 0 errado: %+v col=%d", ls[0], ls[0].Column())
	}
	if ls[1].HasPct || ls[1].Column() != 2 {
		t.Errorf("limite 1 errado: %+v col=%d", ls[1], ls[1].Column())
	}
}

func TestEmailOfNaoVazaNemAceitaLixo(t *testing.T) {
	m := setup(t)
	if EmailOf(m.Live) != "" {
		t.Error("sem arquivo deveria ser vazio")
	}
	os.WriteFile(m.Live, []byte(`{"id_token_email":"../../etc/passwd"}`), 0o600)
	if EmailOf(m.Live) != "" {
		t.Error("e-mail com caminho deveria ser rejeitado")
	}
}

func TestRegisterSwitchRoundtrip(t *testing.T) {
	m := setup(t)
	login(t, m, "a@teste.com")
	if _, err := m.RegisterLive(); err != nil {
		t.Fatal(err)
	}
	login(t, m, "b@teste.com")
	if _, err := m.RegisterLive(); err != nil {
		t.Fatal(err)
	}
	if err := m.SwitchTo("a@teste.com"); err != nil {
		t.Fatal(err)
	}
	if m.ActiveEmail() != "a@teste.com" {
		t.Fatalf("ativa deveria ser a@, é %q", m.ActiveEmail())
	}
	b, _ := os.ReadFile(m.Live)
	if !strings.Contains(string(b), "a-a@teste.com") {
		t.Error("conteúdo do login não veio do slot")
	}
	st, _ := os.Stat(m.Live)
	if st.Mode().Perm() != 0o600 {
		t.Errorf("modo do login = %v, queria 600", st.Mode().Perm())
	}
	if got := m.Accounts(); len(got) != 2 {
		t.Errorf("contas = %v", got)
	}
}

func TestTrocaRecusadaSeLoginAtualNaoEstaGuardado(t *testing.T) {
	m := setup(t)
	login(t, m, "a@teste.com")
	m.RegisterLive()
	login(t, m, "solto@teste.com") // login que o mgy não conhece
	if err := m.SwitchTo("a@teste.com"); err == nil {
		t.Fatal("deveria recusar: perderia o login solto")
	}
	if m.ActiveEmail() != "solto@teste.com" {
		t.Error("o login atual não pode ter sido tocado")
	}
}

func TestTrocaRecusadaComAgyAberto(t *testing.T) {
	m := setup(t)
	login(t, m, "a@teste.com")
	m.RegisterLive()
	login(t, m, "b@teste.com")
	m.RegisterLive()
	agyRunning = func() bool { return true }
	if err := m.SwitchTo("a@teste.com"); err != ErrAgyRunning {
		t.Fatalf("queria ErrAgyRunning, veio %v", err)
	}
}

func TestRefreshAllRestauraOLoginOriginal(t *testing.T) {
	m := setup(t)
	login(t, m, "a@teste.com")
	m.RegisterLive()
	login(t, m, "ruim@teste.com") // conta cujo agy falha
	m.RegisterLive()
	login(t, m, "c@teste.com")
	m.RegisterLive()
	before, _ := os.ReadFile(m.Live)

	warns, err := m.RefreshAll()
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(m.Live)
	if string(before) != string(after) {
		t.Fatal("o login original não foi restaurado")
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "ruim@teste.com") {
		t.Errorf("avisos = %v", warns)
	}
	for _, e := range []string{"a@teste.com", "c@teste.com"} {
		if q := m.LoadQuota(e); len(q.Limits) != 4 || q.At.IsZero() {
			t.Errorf("%s sem cota: %+v", e, q)
		}
	}
}

func TestAdicionarContaNova(t *testing.T) {
	m := setup(t)
	login(t, m, "a@teste.com")
	s, err := m.BeginAdd()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(m.Live); err == nil {
		t.Fatal("o login deveria ter sido limpo para o agy pedir login")
	}
	if !m.Has("a@teste.com") {
		t.Fatal("o login atual deveria ter sido guardado antes de limpar")
	}
	exec := func() { // roda o agy de mentira "interativo": ele loga como novo@
		if err := runFake(m); err != nil {
			t.Fatal(err)
		}
	}
	exec()
	got, err := s.Finish()
	if err != nil || got != "novo@teste.com" {
		t.Fatalf("Finish = %q, %v", got, err)
	}
	if !m.Has("novo@teste.com") || !m.Has("a@teste.com") {
		t.Error("as duas contas deveriam estar guardadas")
	}
	if len(m.LoadQuota("novo@teste.com").Limits) == 0 {
		t.Error("deveria ter lido a cota da conta nova")
	}
}

func TestAdicionarCanceladoRestauraOAnterior(t *testing.T) {
	m := setup(t)
	login(t, m, "a@teste.com")
	s, err := m.BeginAdd()
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Finish() // usuário saiu sem logar
	if err != nil || got != "" {
		t.Fatalf("Finish = %q, %v", got, err)
	}
	if m.ActiveEmail() != "a@teste.com" {
		t.Errorf("deveria restaurar a@, ativa = %q", m.ActiveEmail())
	}
}

func TestLockExclusivo(t *testing.T) {
	m := setup(t)
	un, err := m.Lock()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Lock(); err == nil {
		t.Error("segundo lock deveria falhar")
	}
	un()
	un2, err := m.Lock()
	if err != nil {
		t.Fatalf("depois de liberar deveria travar: %v", err)
	}
	un2()
}

func TestRemove(t *testing.T) {
	m := setup(t)
	login(t, m, "a@teste.com")
	m.RegisterLive()
	m.FetchQuota("a@teste.com")
	if err := m.Remove("a@teste.com"); err != nil {
		t.Fatal(err)
	}
	if m.Has("a@teste.com") || len(m.LoadQuota("a@teste.com").Limits) != 0 {
		t.Error("conta e cota deveriam ter sumido")
	}
	if m.ActiveEmail() != "a@teste.com" {
		t.Error("remover não pode mexer no login ativo do agy")
	}
}
