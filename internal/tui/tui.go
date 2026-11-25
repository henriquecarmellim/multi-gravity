// Package tui é a tela do mgy: contas do agy, cotas e troca, tudo num lugar só.
package tui

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/henriquecarmellim/multi-gravity/internal/core"
)

// Paleta do rice (vermelho sobre fundo escuro).
var (
	cRed   = lipgloss.Color("#ff2d39")
	cRed2  = lipgloss.Color("#ff5b64")
	cRed3  = lipgloss.Color("#ff858c")
	cFg    = lipgloss.Color("#e6e6ea")
	cDim   = lipgloss.Color("#7a7a85")
	cGreen = lipgloss.Color("#7ee0a3")

	sTitle  = lipgloss.NewStyle().Foreground(cRed).Bold(true)
	sDim    = lipgloss.NewStyle().Foreground(cDim)
	sFg     = lipgloss.NewStyle().Foreground(cFg)
	sBold   = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Bold(true)
	sKey    = lipgloss.NewStyle().Foreground(cRed2).Bold(true)
	sWarn   = lipgloss.NewStyle().Foreground(cRed3)
	sErr    = lipgloss.NewStyle().Foreground(cRed).Bold(true)
	sOk     = lipgloss.NewStyle().Foreground(cGreen)
	sPanel  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cRed).Padding(0, 1)
	sBorder = lipgloss.NewStyle().Foreground(cRed)
)

type state int

const (
	stList state = iota
	stAddInfo
	stRmConfirm
)

type (
	refreshedMsg struct {
		warns []string
		err   error
	}
	addExecMsg struct{ err error }
	addDoneMsg struct {
		email string
		err   error
	}
)

type model struct {
	m      *core.Mgr
	accts  []string
	quotas map[string]core.Quota
	active string
	cur    int
	w, h   int
	st     state
	busy   string
	spin   spinner.Model
	note   string
	noteOK bool
	launch string
	sess   *core.AddSession
}

// Run abre o TUI. Devolve a conta escolhida para abrir no agy ("" se saiu sem escolher).
func Run(m *core.Mgr) (string, error) {
	fm, err := tea.NewProgram(newModel(m), tea.WithAltScreen()).Run()
	if err != nil {
		return "", err
	}
	return fm.(model).launch, nil
}

func newModel(m *core.Mgr) model {
	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = sKey
	md := model{m: m, spin: sp, w: 100, h: 30}
	md.reload()
	for i, e := range md.accts { // começa em cima da conta ativa
		if e == md.active {
			md.cur = i
		}
	}
	return md
}

func (md *model) reload() {
	md.accts = md.m.Accounts()
	md.active = md.m.ActiveEmail()
	md.quotas = map[string]core.Quota{}
	for _, e := range md.accts {
		md.quotas[e] = md.m.LoadQuota(e)
	}
	if md.cur >= len(md.accts) {
		md.cur = len(md.accts) - 1
	}
	if md.cur < 0 {
		md.cur = 0
	}
}

func (md model) selected() string {
	if md.cur < len(md.accts) {
		return md.accts[md.cur]
	}
	return ""
}

func (md model) Init() tea.Cmd {
	return tea.Batch(md.spin.Tick, func() tea.Msg {
		return refreshedMsg{err: md.m.RefreshActive()}
	})
}

func (md *model) setNote(s string, ok bool) { md.note, md.noteOK = s, ok }

func (md model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		md.w, md.h = msg.Width, msg.Height
	case spinner.TickMsg:
		var c tea.Cmd
		md.spin, c = md.spin.Update(msg)
		return md, c

	case refreshedMsg:
		md.busy = ""
		md.reload()
		switch {
		case msg.err != nil:
			md.setNote("cota: "+msg.err.Error(), false)
		case len(msg.warns) > 0:
			md.setNote(strings.Join(msg.warns, " · "), false)
		}
	case addExecMsg: // o agy fechou: guarda a conta nova
		md.busy = "guardando a conta…"
		s := md.sess
		return md, func() tea.Msg {
			e, err := s.Finish()
			return addDoneMsg{e, err}
		}
	case addDoneMsg:
		md.busy, md.sess = "", nil
		md.reload()
		switch {
		case msg.err != nil:
			md.setNote(msg.err.Error(), false)
		case msg.email == "":
			md.setNote("nenhum login novo — a conta anterior foi restaurada", false)
		default:
			for i, e := range md.accts {
				if e == msg.email {
					md.cur = i
				}
			}
			md.setNote("conta adicionada: "+msg.email, true)
		}

	case tea.KeyMsg:
		if md.busy != "" {
			return md, nil // ocupado: troca de login em andamento, não interrompe
		}
		return md.key(msg)
	}
	return md, nil
}

func (md model) key(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := k.String()
	switch md.st {
	case stAddInfo:
		switch s {
		case "enter":
			sess, err := md.m.BeginAdd()
			if err != nil {
				md.st = stList
				md.setNote(err.Error(), false)
				return md, nil
			}
			md.sess, md.st = sess, stList
			return md, tea.ExecProcess(exec.Command(md.m.Agy), func(err error) tea.Msg { return addExecMsg{err} })
		case "esc", "q", "n":
			md.st = stList
		}
		return md, nil

	case stRmConfirm:
		if s == "s" || s == "y" {
			e := md.selected()
			if err := md.m.Remove(e); err != nil {
				md.setNote(err.Error(), false)
			} else {
				md.setNote("conta removida: "+e, true)
			}
			md.reload()
		}
		md.st = stList
		return md, nil
	}

	md.note = ""
	switch s {
	case "q", "esc", "ctrl+c":
		return md, tea.Quit
	case "up", "k":
		if md.cur > 0 {
			md.cur--
		}
	case "down", "j":
		if md.cur < len(md.accts)-1 {
			md.cur++
		}
	case "home", "g":
		md.cur = 0
	case "end", "G":
		md.cur = len(md.accts) - 1
		if md.cur < 0 {
			md.cur = 0
		}
	case "enter":
		e := md.selected()
		if e == "" {
			return md, nil
		}
		if err := md.m.SwitchTo(e); err != nil {
			md.setNote(err.Error(), false)
			return md, nil
		}
		md.launch = e
		return md, tea.Quit
	case "a":
		md.st = stAddInfo
	case "x", "delete":
		if md.selected() != "" {
			md.st = stRmConfirm
		}
	case "r":
		md.busy = "lendo a cota de todas as contas…"
		return md, func() tea.Msg {
			w, err := md.m.RefreshAll()
			return refreshedMsg{warns: w, err: err}
		}
	}
	return md, nil
}

// ── desenho ──────────────────────────────────────────────────────────────────

func padR(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

func padL(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return strings.Repeat(" ", d) + s
	}
	return s
}

func clip(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	return string(r[:w-1]) + "…"
}

func pctStyle(p int) lipgloss.Style {
	switch {
	case p <= 10:
		return sErr
	case p <= 50:
		return sWarn
	}
	return sFg
}

func bar(p, n int) string {
	f := (p*n + 50) / 100
	st := pctStyle(p)
	return st.Render(strings.Repeat("█", f)) + sDim.Render(strings.Repeat("░", n-f))
}

func resetShort(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	t = t.Local()
	if t.YearDay() == time.Now().YearDay() && t.Year() == time.Now().Year() {
		return "↻" + t.Format("15:04")
	}
	return "↻" + t.Format("02/01")
}

func age(t time.Time) string {
	if t.IsZero() {
		return "sem dados"
	}
	d := time.Since(t)
	switch {
	case d < 90*time.Second:
		return "agora"
	case d < time.Hour:
		return fmt.Sprintf("há %dmin", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("há %dh", int(d.Hours()))
	}
	return fmt.Sprintf("há %dd", int(d.Hours()/24))
}

// cell desenha uma coluna de cota. Larguras: cheio = 8+1+4+1+5, compacto = 4+1+5.
func cell(q core.Quota, col int, full bool) string {
	var lim *core.Limit
	for i := range q.Limits {
		if q.Limits[i].Column() == col {
			lim = &q.Limits[i]
		}
	}
	w := 10
	if full {
		w = 19
	}
	if lim == nil {
		return padR(sDim.Render("—"), w)
	}
	if !lim.HasPct {
		return padR(sDim.Render("desativado"), w)
	}
	pct := pctStyle(lim.Pct).Render(padL(fmt.Sprintf("%d%%", lim.Pct), 4))
	rs := sDim.Render(padR(resetShort(lim.Reset), 5))
	if full {
		return bar(lim.Pct, 8) + " " + pct + " " + rs
	}
	return pct + " " + rs
}

func (md model) View() string {
	var b strings.Builder
	b.WriteString("\n")
	title := sTitle.Render(" ◆ multi-gravity") + sDim.Render("  contas do agy")
	right := sDim.Render(fmt.Sprintf("%d conta(s) ", len(md.accts)))
	gap := md.w - lipgloss.Width(title) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	b.WriteString(title + strings.Repeat(" ", gap) + right + "\n\n")

	switch md.st {
	case stAddInfo:
		b.WriteString(md.addInfo())
	default:
		b.WriteString(md.table())
	}

	b.WriteString("\n")
	switch {
	case md.busy != "":
		b.WriteString(" " + md.spin.View() + " " + sWarn.Render(md.busy) + "\n")
	case md.st == stRmConfirm:
		b.WriteString(" " + sWarn.Render("remover "+md.selected()+" do mgy?") + sDim.Render(" (o login só é esquecido aqui) ") + sKey.Render("s") + sDim.Render("/n") + "\n")
	case md.note != "":
		st := sErr
		if md.noteOK {
			st = sOk
		}
		b.WriteString(" " + st.Render(clip(md.note, md.w-3)) + "\n")
	default:
		b.WriteString("\n")
	}
	b.WriteString(md.footer())
	return b.String()
}

func (md model) addInfo() string {
	t := sBold.Render("Adicionar conta") + "\n\n" +
		sKey.Render("1.") + sFg.Render(" o agy vai abrir pedindo login: entre com a conta ") + sBold.Render("nova") + sFg.Render(" no navegador") + "\n" +
		sKey.Render("2.") + sFg.Render(" depois saia do agy (") + sKey.Render("/exit") + sFg.Render(" ou ctrl+c) e o mgy guarda a conta") + "\n\n" +
		sDim.Render("o login atual fica guardado e volta se você cancelar")
	return "  " + strings.ReplaceAll(sPanel.Render(t), "\n", "\n  ") + "\n"
}

func (md model) table() string {
	if len(md.accts) == 0 {
		return "  " + sFg.Render("nenhuma conta ainda — aperte ") + sKey.Render("a") + sFg.Render(" para adicionar a primeira") + "\n"
	}
	emailW := 12
	for _, e := range md.accts {
		if len(e) > emailW {
			emailW = len(e)
		}
	}
	if emailW > 34 {
		emailW = 34
	}
	full := md.w >= emailW+8+4*19+3*2
	names := [4]string{"Gemini 5h", "Gemini sem", "Claude/GPT 5h", "Claude/GPT sem"}
	cw, sep := 19, "  "
	if !full {
		names = [4]string{"Gem 5h", "Gem sem", "C/GPT 5h", "C/GPT sem"}
		cw = 10
	}
	var b strings.Builder
	b.WriteString("    " + padR("", emailW) + "  ")
	for i, n := range names {
		b.WriteString(sDim.Render(padR(n, cw)))
		if i < 3 {
			b.WriteString(sep)
		}
	}
	b.WriteString("\n")
	for i, e := range md.accts {
		cur, dot := "  ", sDim.Render("○")
		es := sFg
		if i == md.cur {
			cur, es = sTitle.Render("▶ "), sBold
		}
		if e == md.active {
			dot = sTitle.Render("●")
		}
		line := " " + cur + dot + " " + es.Render(padR(clip(e, emailW), emailW)) + "  "
		q := md.quotas[e]
		for c := 0; c < 4; c++ {
			line += cell(q, c, full)
			if c < 3 {
				line += sep
			}
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" + md.detail())
	return b.String()
}

func (md model) detail() string {
	e := md.selected()
	if e == "" {
		return ""
	}
	q := md.quotas[e]
	head := sBold.Render(e)
	if e == md.active {
		head += "  " + sTitle.Render("(ativa no agy)")
	}
	var l []string
	l = append(l, head+sDim.Render("   cota lida "+age(q.At)))
	if len(q.Limits) == 0 {
		l = append(l, sDim.Render("sem dados — aperte ")+sKey.Render("r")+sDim.Render(" para atualizar"))
	}
	for c := 0; c < 4; c++ {
		for _, lim := range q.Limits {
			if lim.Column() != c {
				continue
			}
			v := sDim.Render("desativado")
			if lim.HasPct {
				v = bar(lim.Pct, 12) + " " + pctStyle(lim.Pct).Render(padL(fmt.Sprintf("%d%%", lim.Pct), 4))
			}
			rs := ""
			if !lim.Reset.IsZero() {
				rs = sDim.Render("   renova " + lim.Reset.Local().Format("02/01 15:04"))
			}
			l = append(l, sFg.Render(padR(core.ColumnNames[c], 15))+" "+v+rs)
		}
	}
	return "  " + strings.ReplaceAll(sPanel.Render(strings.Join(l, "\n")), "\n", "\n  ") + "\n"
}

func (md model) footer() string {
	k := func(key, label string) string { return sKey.Render(key) + sDim.Render(" "+label) }
	var parts []string
	switch md.st {
	case stAddInfo:
		parts = []string{k("↵", "abrir o agy"), k("esc", "voltar")}
	case stRmConfirm:
		parts = []string{k("s", "remover"), k("n", "cancelar")}
	default:
		parts = []string{k("↑↓", "mover"), k("↵", "trocar e abrir o agy"), k("a", "adicionar"), k("x", "remover"), k("r", "atualizar cotas"), k("q", "sair")}
	}
	return " " + strings.Join(parts, sDim.Render("  ·  ")) + "\n"
}
