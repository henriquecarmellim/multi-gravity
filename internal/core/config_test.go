package core

import (
	"reflect"
	"testing"
	"time"
)

func q(g5, gw, c5, cw int, reset time.Time) Quota {
	mk := func(group, label string, p int) Limit {
		return Limit{Group: group, Label: label, Pct: p, HasPct: p >= 0, Reset: reset}
	}
	return Quota{Limits: []Limit{
		mk("Gemini Models", "Five Hour Limit Remaining", g5),
		mk("Gemini Models", "Weekly Limit Remaining", gw),
		mk("Claude and GPT models", "Five Hour Limit Remaining", c5),
		mk("Claude and GPT models", "Weekly Limit Remaining", cw),
	}}
}

func TestScoreUsaAMenorJanela(t *testing.T) {
	future := time.Now().Add(time.Hour)
	c := DefaultConfig()
	if s := c.Score(q(100, 6, 80, 80, future), time.Now()); s != 6 {
		t.Errorf("gemini = %d, queria 6 (a semanal trava)", s)
	}
	c.Family = "claude"
	if s := c.Score(q(100, 6, 80, 40, future), time.Now()); s != 40 {
		t.Errorf("claude = %d, queria 40", s)
	}
	c.Family = "both"
	if s := c.Score(q(100, 60, 80, 40, future), time.Now()); s != (60+40)/2 {
		t.Errorf("both = %d, queria 50", s)
	}
	if s := c.Score(Quota{}, time.Now()); s != -1 {
		t.Errorf("sem dados = %d, queria -1", s)
	}
}

func TestJanelaJaRenovadaContaComo100(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	if s := DefaultConfig().Score(q(0, 0, 0, 0, past), time.Now()); s != 100 {
		t.Errorf("cache velho com renovação passada = %d, queria 100", s)
	}
}

func TestClaudeComJanelaDesativadaIgnoraEla(t *testing.T) {
	future := time.Now().Add(time.Hour)
	c := DefaultConfig()
	c.Family = "claude"
	if s := c.Score(q(100, 100, -1, 30, future), time.Now()); s != 30 {
		t.Errorf("claude com 5h desativada = %d, queria 30", s)
	}
}

func TestOrderedEBest(t *testing.T) {
	m := setup(t)
	for _, e := range []string{"a@teste.com", "b@teste.com", "c@teste.com", "d@teste.com"} {
		login(t, m, e)
		m.RegisterLive()
	}
	future := time.Now().Add(time.Hour)
	put := func(e string, qq Quota) {
		var s string
		for _, l := range qq.Limits {
			p := "disabled"
			if l.HasPct {
				p = itoa(l.Pct) + "%"
			}
			s += l.Group + "\t" + l.Label + "\t" + p + "\t" + future.UTC().Format(time.RFC3339) + "\n"
		}
		writeQuota(t, m, e, s)
	}
	put("a@teste.com", q(100, 6, 100, 100, future))  // gemini 6
	put("b@teste.com", q(100, 90, 100, 100, future)) // gemini 90  <- melhor
	put("c@teste.com", q(50, 70, 100, 100, future))  // gemini 50
	// d sem dados

	c := DefaultConfig()
	got := m.Ordered(c)
	want := []string{"b@teste.com", "c@teste.com", "a@teste.com", "d@teste.com"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ordem = %v, queria %v", got, want)
	}
	if b := m.Best(c); b != "b@teste.com" {
		t.Errorf("best = %q", b)
	}
	c.Strategy = "manual"
	if got := m.Ordered(c); got[0] != "a@teste.com" {
		t.Errorf("manual deveria ser alfabética: %v", got)
	}
	// todas zeradas: sem melhor
	for _, e := range m.Accounts() {
		put(e, q(0, 0, 0, 0, future))
	}
	if b := m.Best(DefaultConfig()); b != "" {
		t.Errorf("todas zeradas: best = %q, queria vazio", b)
	}
}

func TestConfigPersisteESanitiza(t *testing.T) {
	m := setup(t)
	if c := m.LoadConfig(); c != DefaultConfig() {
		t.Errorf("sem arquivo deveria ser o padrão: %+v", c)
	}
	c := DefaultConfig()
	if err := c.Set("skip-permissions", "on"); err != nil {
		t.Fatal(err)
	}
	if err := c.Set("family", "claude"); err != nil {
		t.Fatal(err)
	}
	if err := m.SaveConfig(c); err != nil {
		t.Fatal(err)
	}
	if got := m.LoadConfig(); !got.SkipPermissions || got.Family != "claude" {
		t.Errorf("não persistiu: %+v", got)
	}
	if err := c.Set("family", "banana"); err == nil {
		t.Error("valor inválido deveria dar erro")
	}
	if err := c.Set("nada", "on"); err == nil {
		t.Error("opção desconhecida deveria dar erro")
	}
	writeFile(t, m.configFile(), `{"strategy":"lixo","family":"x","continue":false}`)
	if got := m.LoadConfig(); got.Strategy != "best" || got.Family != "gemini" || got.Continue {
		t.Errorf("deveria sanear enums e respeitar o continue=false: %+v", got)
	}
}

func TestLaunchArgs(t *testing.T) {
	c := DefaultConfig() // continue ligado, permissões desligadas
	if got := c.LaunchArgs(nil); !reflect.DeepEqual(got, []string{"--continue"}) {
		t.Errorf("padrão = %v", got)
	}
	c.SkipPermissions = true
	if got := c.LaunchArgs([]string{"--effort", "high"}); !reflect.DeepEqual(got, []string{"--dangerously-skip-permissions", "--continue", "--effort", "high"}) {
		t.Errorf("com permissões = %v", got)
	}
	if got := c.LaunchArgs([]string{"-r", "abc"}); !reflect.DeepEqual(got, []string{"--dangerously-skip-permissions", "-r", "abc"}) {
		t.Errorf("o usuário escolheu a conversa, o mgy não põe --continue: %v", got)
	}
	if got := c.LaunchArgs([]string{"--dangerously-skip-permissions"}); !reflect.DeepEqual(got, []string{"--continue", "--dangerously-skip-permissions"}) {
		t.Errorf("sem duplicar a flag: %v", got)
	}
	c = Config{Strategy: "best", Family: "gemini"} // tudo desligado
	if got := c.LaunchArgs(nil); len(got) != 0 {
		t.Errorf("desligado = %v", got)
	}
}

func TestLaunchArgsComPrintModeTambemContinua(t *testing.T) {
	got := DefaultConfig().LaunchArgs([]string{"-p", "oi"})
	if !reflect.DeepEqual(got, []string{"--continue", "-p", "oi"}) {
		t.Errorf("-p deveria continuar a conversa: %v", got)
	}
}
