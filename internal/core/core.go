// Package core guarda as contas do Antigravity CLI (agy), troca o login ativo
// e lê as cotas de cada conta.
//
// O agy autentica pelo arquivo antigravity-oauth-token. O mgy guarda uma cópia
// desse arquivo por conta (modo 600) e troca copiando a escolhida de volta.
// A cota vem do próprio agy: `agy -p "/quota"`.
package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	ErrAgyRunning  = errors.New("o agy está aberto — feche-o antes de trocar de conta")
	ErrNoAccount   = errors.New("conta não cadastrada")
	ErrNotLoggedIn = errors.New("nenhum login válido no agy")
	emailRe        = regexp.MustCompile(`^[A-Za-z0-9._%+@-]+$`)
)

// Mgr concentra os caminhos. Podem ser trocados por variáveis de ambiente
// (MGY_LIVE, MGY_STORE, MGY_AGY), o que também facilita os testes.
type Mgr struct {
	Live  string // arquivo de login do agy
	Store string // pasta do mgy
	Agy   string // binário do agy
}

func New() *Mgr {
	home, _ := os.UserHomeDir()
	m := &Mgr{
		Live:  os.Getenv("MGY_LIVE"),
		Store: os.Getenv("MGY_STORE"),
		Agy:   os.Getenv("MGY_AGY"),
	}
	if m.Live == "" {
		m.Live = filepath.Join(home, ".gemini", "antigravity-cli", "antigravity-oauth-token")
	}
	if m.Store == "" {
		m.Store = filepath.Join(dataHome(home), "mgy")
	}
	if m.Agy == "" {
		if p, err := exec.LookPath("agy"); err == nil {
			m.Agy = p
		} else {
			m.Agy = filepath.Join(home, ".local", "bin", "agy"+exeSuffix)
		}
	}
	return m
}

// agyRunning pode ser trocada nos testes.
var agyRunning = AgyRunning

func (m *Mgr) accDir() string   { return filepath.Join(m.Store, "accounts") }
func (m *Mgr) quotaDir() string { return filepath.Join(m.Store, "quota") }
func (m *Mgr) slot(email string) string {
	return filepath.Join(m.accDir(), email+".json")
}

// Init cria as pastas privadas (700).
func (m *Mgr) Init() error {
	for _, d := range []string{m.Store, m.accDir(), m.quotaDir()} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return err
		}
		_ = os.Chmod(d, 0o700)
	}
	return nil
}

// Lock impede duas instâncias de mexerem no login ao mesmo tempo.
func (m *Mgr) Lock() (unlock func(), err error) {
	if err = m.Init(); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(m.Store, ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err = lockFile(f); err != nil {
		f.Close()
		return nil, errors.New("outra instância do mgy está rodando")
	}
	return func() { unlockFile(f); f.Close() }, nil
}

// ── login ────────────────────────────────────────────────────────────────────

// EmailOf lê o e-mail dentro de um arquivo de login, sem expor o token.
func EmailOf(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var t struct {
		Email string `json:"id_token_email"`
	}
	if json.Unmarshal(b, &t) != nil {
		return ""
	}
	if !ValidEmail(t.Email) {
		return ""
	}
	return t.Email
}

func ValidEmail(e string) bool {
	return e != "" && emailRe.MatchString(e) && !strings.Contains(e, "..")
}

// ActiveEmail é a conta que está ativa no agy agora ("" se não há login).
func (m *Mgr) ActiveEmail() string { return EmailOf(m.Live) }

// Accounts devolve os e-mails guardados, em ordem.
func (m *Mgr) Accounts() []string {
	ents, _ := os.ReadDir(m.accDir())
	var out []string
	for _, e := range ents {
		if n, ok := strings.CutSuffix(e.Name(), ".json"); ok && ValidEmail(n) {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

func (m *Mgr) Has(email string) bool {
	_, err := os.Stat(m.slot(email))
	return err == nil
}

// copyAtomic copia src para dst (modo 600) trocando o arquivo por rename,
// nunca por symlink: o agy pode substituir o arquivo por conta própria.
func copyAtomic(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".mgy-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err = tmp.Chmod(0o600); err == nil {
		_, err = tmp.Write(b)
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, dst)
}

// RegisterLive guarda o login atual do agy como conta e devolve o e-mail.
func (m *Mgr) RegisterLive() (string, error) {
	e := m.ActiveEmail()
	if e == "" {
		return "", ErrNotLoggedIn
	}
	if err := m.Init(); err != nil {
		return "", err
	}
	return e, copyAtomic(m.Live, m.slot(e))
}

// SaveLive grava o login vivo de volta no slot da conta (o agy pode renovar o token).
func (m *Mgr) SaveLive() {
	if e := m.ActiveEmail(); e != "" && m.Has(e) {
		_ = copyAtomic(m.Live, m.slot(e))
	}
}

func (m *Mgr) install(email string) error {
	if !m.Has(email) {
		return ErrNoAccount
	}
	return copyAtomic(m.slot(email), m.Live)
}

// CanSwap: só troca com o agy fechado e sem perder o login atual.
func (m *Mgr) CanSwap() error {
	if agyRunning() {
		return ErrAgyRunning
	}
	if _, err := os.Stat(m.Live); err == nil {
		e := m.ActiveEmail()
		if e != "" && !m.Has(e) {
			return fmt.Errorf("o login atual (%s) não está guardado", e)
		}
	}
	return nil
}

// SwitchTo deixa a conta escolhida ativa no agy.
func (m *Mgr) SwitchTo(email string) error {
	if !m.Has(email) {
		return ErrNoAccount
	}
	if m.ActiveEmail() == email {
		return nil
	}
	unlock, err := m.Lock() // evita brigar com um `mgy quota` rodando em outro terminal
	if err != nil {
		return err
	}
	defer unlock()
	if err := m.CanSwap(); err != nil {
		return err
	}
	m.SaveLive()
	return m.install(email)
}

// Remove esquece uma conta (o login ativo do agy não é tocado).
func (m *Mgr) Remove(email string) error {
	if !ValidEmail(email) || !m.Has(email) {
		return ErrNoAccount
	}
	_ = os.Remove(m.quotaFile(email))
	return os.Remove(m.slot(email))
}

// ── adicionar conta ──────────────────────────────────────────────────────────

// AddSession é uma adição de conta em andamento: o login atual já está guardado
// e o arquivo do agy foi limpo para ele pedir login.
type AddSession struct {
	m      *Mgr
	orig   string
	unlock func()
}

// BeginAdd guarda o login atual, limpa o arquivo e segura o lock.
// Depois disso, rode o agy interativo e chame Finish.
func (m *Mgr) BeginAdd() (*AddSession, error) {
	if agyRunning() {
		return nil, ErrAgyRunning
	}
	unlock, err := m.Lock()
	if err != nil {
		return nil, err
	}
	s := &AddSession{m: m, unlock: unlock}
	if _, err := os.Stat(m.Live); err == nil {
		if e, rerr := m.RegisterLive(); rerr == nil {
			s.orig = e
		} else { // login sem e-mail legível: não perde, guarda de lado
			_ = copyAtomic(m.Live, filepath.Join(m.Store, ".login-anterior.bak"))
		}
		if err := os.Remove(m.Live); err != nil {
			unlock()
			return nil, err
		}
	}
	return s, nil
}

// Finish registra o login novo (e lê a cota). Sem login novo, restaura o anterior.
// Devolve o e-mail da conta adicionada ("" se nada foi adicionado).
func (s *AddSession) Finish() (string, error) {
	defer s.unlock()
	if e, err := s.m.RegisterLive(); err == nil {
		_ = s.m.FetchQuota(e)
		return e, nil
	}
	if s.orig != "" {
		if err := s.m.install(s.orig); err != nil {
			return "", err
		}
	}
	return "", nil
}

// ── cota ─────────────────────────────────────────────────────────────────────

type Limit struct {
	Group, Label string
	Pct          int  // 0..100, válido se HasPct
	HasPct       bool // false = "disabled"
	Reset        time.Time
}

type Quota struct {
	Limits []Limit
	At     time.Time // quando foi lida (mtime do cache)
}

// Column: 0 Gemini 5h · 1 Gemini sem · 2 Claude/GPT 5h · 3 Claude/GPT sem.
func (l Limit) Column() int {
	c := 2
	if strings.Contains(l.Group, "Gemini") {
		c = 0
	}
	if !strings.Contains(l.Label, "Five") {
		c++
	}
	return c
}

var ColumnNames = [4]string{"Gemini 5h", "Gemini sem", "Claude/GPT 5h", "Claude/GPT sem"}

// ParseQuota lê a saída TSV de `agy -p "/quota"`.
func ParseQuota(s string) []Limit {
	var out []Limit
	for _, line := range strings.Split(s, "\n") {
		f := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(f) < 3 || f[0] == "" {
			continue
		}
		l := Limit{Group: f[0], Label: f[1]}
		if n, err := strconv.Atoi(strings.TrimSuffix(f[2], "%")); err == nil && strings.HasSuffix(f[2], "%") {
			l.Pct, l.HasPct = n, true
		}
		if len(f) > 3 && f[3] != "" {
			l.Reset, _ = time.Parse(time.RFC3339, f[3])
		}
		out = append(out, l)
	}
	return out
}

func (m *Mgr) quotaFile(email string) string {
	return filepath.Join(m.quotaDir(), email+".tsv")
}

// LoadQuota lê a cota guardada em cache (vazia se nunca foi lida).
func (m *Mgr) LoadQuota(email string) Quota {
	p := m.quotaFile(email)
	b, err := os.ReadFile(p)
	if err != nil {
		return Quota{}
	}
	q := Quota{Limits: ParseQuota(string(b))}
	if st, err := os.Stat(p); err == nil {
		q.At = st.ModTime()
	}
	return q
}

// FetchQuota lê a cota da conta que está ativa no agy e guarda em cache.
func (m *Mgr) FetchQuota(email string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, m.Agy, "-p", "/quota").Output()
	if err != nil && len(out) == 0 {
		return fmt.Errorf("agy não respondeu: %w", err)
	}
	if len(ParseQuota(string(out))) == 0 {
		return errors.New("sem dados de cota (login expirado?)")
	}
	if err := m.Init(); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(m.quotaDir(), ".q-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	_, werr := tmp.Write(out)
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return werr
	}
	return os.Rename(name, m.quotaFile(email))
}

// StaleActive diz se a cota da conta ativa precisa ser lida (cache com mais de 5 minutos).
func (m *Mgr) StaleActive() bool {
	e := m.ActiveEmail()
	if e == "" {
		return false
	}
	q := m.LoadQuota(e)
	return q.At.IsZero() || time.Since(q.At) >= 5*time.Minute
}

// RefreshActive atualiza só a conta ativa, se o cache estiver velho.
func (m *Mgr) RefreshActive() error {
	if !m.StaleActive() {
		return nil
	}
	return m.FetchQuota(m.ActiveEmail())
}
