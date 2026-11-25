package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/henriquecarmellim/multi-gravity/internal/core"
)

func setup(t *testing.T) *core.Mgr {
	t.Helper()
	dir := t.TempDir()
	m := &core.Mgr{Live: filepath.Join(dir, "live"), Store: filepath.Join(dir, "store"), Agy: "agy"}
	if err := m.Init(); err != nil {
		t.Fatal(err)
	}
	for _, e := range []string{"a@teste.com", "b@teste.com"} {
		os.WriteFile(m.Live, []byte(`{"token":{},"id_token_email":"`+e+`"}`), 0o600)
		if _, err := m.RegisterLive(); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(dir, "store", "quota", "a@teste.com.tsv"), []byte(
		"Gemini Models\tWeekly Limit Remaining\t6%\t2026-11-30T20:10:30Z\n"+
			"Gemini Models\tFive Hour Limit Remaining\t100%\t2026-11-25T18:32:53Z\n"+
			"Claude and GPT models\tWeekly Limit Remaining\t0%\t2026-11-25T16:51:04Z\n"+
			"Claude and GPT models\tFive Hour Limit Remaining\tdisabled\t\n"), 0o600)
	return m
}

func press(md tea.Model, keys ...string) tea.Model {
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		md, _ = md.Update(msg)
	}
	return md
}

func TestViewMostraContasECotas(t *testing.T) {
	var md tea.Model = newModel(setup(t))
	md, _ = md.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	v := md.View()
	for _, want := range []string{"a@teste.com", "b@teste.com", "6%", "100%", "0%", "desativado", "multi-gravity", "2 conta(s)"} {
		if !strings.Contains(v, want) {
			t.Errorf("a tela não mostra %q:\n%s", want, v)
		}
	}
	md, _ = md.Update(tea.WindowSizeMsg{Width: 70, Height: 30}) // estreito: modo compacto
	if v := md.View(); !strings.Contains(v, "a@teste.com") || !strings.Contains(v, "C/GPT") {
		t.Errorf("modo compacto quebrou:\n%s", v)
	}
}

func TestEnterNaContaAtivaAbreOAgy(t *testing.T) {
	m := setup(t) // a conta ativa é a última registrada: b@
	md := press(newModel(m), "enter").(model)
	if md.launch != "b@teste.com" {
		t.Errorf("launch = %q, queria b@teste.com", md.launch)
	}
}

func TestAdicionarMostraInstrucoesEEscVolta(t *testing.T) {
	md := press(newModel(setup(t)), "a")
	if v := md.View(); !strings.Contains(v, "Adicionar conta") || !strings.Contains(v, "/exit") {
		t.Errorf("faltam as instruções:\n%s", v)
	}
	md = press(md, "esc")
	if strings.Contains(md.View(), "Adicionar conta") {
		t.Error("esc deveria voltar para a lista")
	}
}

func TestRemoverPedeConfirmacao(t *testing.T) {
	m := setup(t)
	md := press(newModel(m), "x", "n") // cancela
	if len(m.Accounts()) != 2 {
		t.Fatal("cancelar não pode remover")
	}
	md = press(md, "x", "s")
	if got := m.Accounts(); len(got) != 1 {
		t.Fatalf("contas = %v", got)
	}
	if !strings.Contains(md.View(), "conta removida") {
		t.Error("deveria avisar que removeu")
	}
}

func TestSemContasConvidaAAdicionar(t *testing.T) {
	dir := t.TempDir()
	m := &core.Mgr{Live: filepath.Join(dir, "live"), Store: filepath.Join(dir, "store"), Agy: "agy"}
	m.Init()
	if v := newModel(m).View(); !strings.Contains(v, "nenhuma conta ainda") {
		t.Errorf("estado vazio:\n%s", v)
	}
}
