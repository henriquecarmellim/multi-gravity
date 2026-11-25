package core

import (
	"sort"
	"time"
)

// famCols: colunas de cota de cada família (0 Gemini 5h, 1 Gemini sem, 2 Claude/GPT 5h, 3 Claude/GPT sem).
var famCols = map[string][]int{
	"gemini": {0, 1},
	"claude": {2, 3},
}

// familyScore é a cota que ainda dá para usar numa família: o MENOR entre as
// janelas (5h e semanal), porque a que acabar primeiro trava a conta.
// Janela cujo horário de renovação já passou conta como 100% (a cota em cache é velha).
func familyScore(q Quota, cols []int, now time.Time) (int, bool) {
	best, ok := 100, false
	for _, l := range q.Limits {
		in := false
		for _, c := range cols {
			if l.Column() == c {
				in = true
			}
		}
		if !in || !l.HasPct {
			continue
		}
		p := l.Pct
		if !l.Reset.IsZero() && l.Reset.Before(now) {
			p = 100
		}
		if !ok || p < best {
			best = p
		}
		ok = true
	}
	return best, ok
}

// Score é a nota da conta de 0 a 100 para a família escolhida; -1 se não há dados.
func (c Config) Score(q Quota, now time.Time) int {
	if c.Family == "both" {
		g, gok := familyScore(q, famCols["gemini"], now)
		cl, cok := familyScore(q, famCols["claude"], now)
		switch {
		case gok && cok:
			return (g + cl) / 2
		case gok:
			return g
		case cok:
			return cl
		}
		return -1
	}
	if s, ok := familyScore(q, famCols[c.Family], now); ok {
		return s
	}
	return -1
}

// Ordered devolve as contas na ordem de uso: pela melhor cota ("best") ou alfabética ("manual").
func (m *Mgr) Ordered(c Config) []string {
	accts := m.Accounts()
	if c.Strategy != "best" {
		return accts
	}
	now := time.Now()
	score := map[string]int{}
	for _, e := range accts {
		score[e] = c.Score(m.LoadQuota(e), now)
	}
	sort.SliceStable(accts, func(i, j int) bool { return score[accts[i]] > score[accts[j]] })
	return accts
}

// Best é a primeira conta da ordem que ainda tem cota ("" se todas zeradas ou sem dados).
func (m *Mgr) Best(c Config) string {
	now := time.Now()
	for _, e := range m.Ordered(c) {
		if c.Score(m.LoadQuota(e), now) > 0 {
			return e
		}
	}
	return ""
}

// NeedsRefreshAny diz se alguma conta está sem cota em cache ou com cache velho (mais de 5 min).
func (m *Mgr) NeedsRefreshAny() bool {
	for _, e := range m.Accounts() {
		q := m.LoadQuota(e)
		if q.At.IsZero() || time.Since(q.At) >= 5*time.Minute {
			return true
		}
	}
	return false
}
