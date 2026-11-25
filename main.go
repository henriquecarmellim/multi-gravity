// mgy — várias contas do Antigravity CLI (agy): cotas e troca por um TUI.
//
//	mgy                abre o TUI
//	mgy -- <args>      abre o TUI e repassa <args> ao agy
//	mgy auto [args]    lê as cotas, escolhe a melhor conta e abre o agy
//	mgy ls             lista as contas e as cotas (do cache)
//	mgy quota          atualiza a cota de todas as contas e lista
//	mgy add            adiciona uma conta (abre o agy para o login)
//	mgy use <n|email>  troca de conta e abre o agy
//	mgy switch <n|email>  só troca de conta (não abre o agy)
//	mgy config [chave valor]  mostra ou muda as opções
//	mgy rm <email>     esquece uma conta
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/henriquecarmellim/multi-gravity/internal/core"
	"github.com/henriquecarmellim/multi-gravity/internal/tui"
)

var version = "dev"

const help = `mgy — várias contas do Antigravity CLI (agy)

uso:
  mgy                 abre o TUI (trocar, adicionar, remover e ver as cotas)
  mgy -- <args>       abre o TUI e repassa <args> ao agy (ex.: mgy -- -c)
  mgy auto [args]     lê as cotas, escolhe a melhor conta e abre o agy
  mgy ls              lista as contas (da melhor para a pior) e as cotas (do cache)
  mgy quota           atualiza a cota de todas as contas e lista
  mgy add             adiciona uma conta (abre o agy para você fazer login)
  mgy use <n|email>   troca de conta e abre o agy
  mgy switch <n|email>  só troca de conta, sem abrir o agy
  mgy config          mostra as opções
  mgy config <chave> <valor>   muda uma opção:
                        strategy          best | manual      (ordem das contas)
                        family            gemini | claude | both   (cota que pesa no "melhor")
                        continue          on | off           (abre o agy com --continue)
                        skip-permissions  on | off           (abre o agy com --dangerously-skip-permissions)
  mgy rm <email>      esquece uma conta
  mgy version

variáveis: MGY_LIVE (arquivo de login do agy), MGY_STORE (pasta do mgy), MGY_AGY (binário do agy)`

func die(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "mgy: "+format+"\n", a...)
	os.Exit(1)
}

func main() {
	m := core.New()
	if err := m.Init(); err != nil {
		die("%v", err)
	}
	args := os.Args[1:]
	cmd := ""
	if len(args) > 0 {
		cmd = args[0]
	}
	switch cmd {
	case "":
		runTUI(m, nil)
	case "--":
		runTUI(m, args[1:])
	case "ls", "list":
		cmdLs(m)
	case "quota", "q":
		warns, err := m.RefreshAll()
		if err != nil {
			die("%v", err)
		}
		for _, w := range warns {
			fmt.Fprintln(os.Stderr, "aviso:", w)
		}
		cmdLs(m)
	case "add":
		cmdAdd(m)
	case "auto":
		cmdAuto(m, args[1:])
	case "switch":
		if len(args) < 2 {
			die("uso: mgy switch <n|email>")
		}
		e := resolve(m, args[1])
		if err := m.SwitchTo(e); err != nil {
			die("%v", err)
		}
		fmt.Println("conta ativa:", e)
	case "config":
		cmdConfig(m, args[1:])
	case "use":
		if len(args) < 2 {
			die("uso: mgy use <n|email> [args do agy]")
		}
		e := resolve(m, args[1])
		if err := m.SwitchTo(e); err != nil {
			die("%v", err)
		}
		launch(m, args[2:])
	case "rm", "remove":
		if len(args) < 2 {
			die("uso: mgy rm <email>")
		}
		if err := m.Remove(args[1]); err != nil {
			die("%v", err)
		}
		fmt.Println("removida:", args[1])
	case "version", "-v", "--version":
		fmt.Println("mgy", version)
	case "help", "-h", "--help":
		fmt.Println(help)
	default:
		die("comando desconhecido: %s (veja: mgy help)", cmd)
	}
}

func resolve(m *core.Mgr, s string) string {
	if n, err := strconv.Atoi(s); err == nil {
		if a := m.Ordered(m.LoadConfig()); n >= 1 && n <= len(a) {
			return a[n-1]
		}
		die("não existe a conta número %d", n)
	}
	return s
}

func launch(m *core.Mgr, args []string) {
	if _, err := exec.LookPath(m.Agy); err != nil {
		die("não achei o agy (%s) — instale-o ou aponte MGY_AGY", m.Agy)
	}
	if err := launchAgy(m.Agy, m.LoadConfig().LaunchArgs(args)); err != nil {
		die("não consegui abrir o agy: %v", err)
	}
}

func runTUI(m *core.Mgr, agyArgs []string) {
	if _, err := exec.LookPath(m.Agy); err != nil {
		die("não achei o agy (%s) — instale-o ou aponte MGY_AGY", m.Agy)
	}
	// o login que já está no agy entra sozinho na lista
	if cur := m.ActiveEmail(); cur != "" && !m.Has(cur) {
		_, _ = m.RegisterLive()
	}
	e, err := tui.Run(m)
	if err != nil {
		die("%v", err)
	}
	if e != "" {
		launch(m, agyArgs)
	}
}

func cmdLs(m *core.Mgr) {
	cfg := m.LoadConfig()
	accts := m.Ordered(cfg)
	if len(accts) == 0 {
		die("nenhuma conta guardada — rode: mgy add")
	}
	live, best := m.ActiveEmail(), m.Best(cfg)
	for i, e := range accts {
		mark := "○"
		if e == live {
			mark = "●"
		}
		star := " "
		if e == best && cfg.Strategy == "best" {
			star = "★"
		}
		q := m.LoadQuota(e)
		var cols []string
		for c := 0; c < 4; c++ {
			v := "—"
			for _, l := range q.Limits {
				if l.Column() == c {
					if l.HasPct {
						v = fmt.Sprintf("%d%%", l.Pct)
					} else {
						v = "off"
					}
				}
			}
			cols = append(cols, fmt.Sprintf("%s %s", core.ColumnNames[c], v))
		}
		fmt.Printf("%d %s%s %-32s %s\n", i+1, mark, star, e, strings.Join(cols, " | "))
	}
}

func cmdAdd(m *core.Mgr) {
	if _, err := exec.LookPath(m.Agy); err != nil {
		die("não achei o agy (%s)", m.Agy)
	}
	s, err := m.BeginAdd()
	if err != nil {
		die("%v", err)
	}
	fmt.Println("O agy vai abrir pedindo login: entre com a conta NOVA.")
	fmt.Println("Depois saia do agy (/exit ou ctrl+c) para o mgy guardar a conta.")
	fmt.Print("Enter para abrir o agy… ")
	_, _ = fmt.Scanln()
	c := exec.Command(m.Agy)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	_ = c.Run()
	e, err := s.Finish()
	switch {
	case err != nil:
		die("%v", err)
	case e == "":
		fmt.Println("nenhum login novo — conta anterior restaurada")
	default:
		fmt.Println("conta adicionada:", e)
	}
}

func cmdAuto(m *core.Mgr, args []string) {
	cfg := m.LoadConfig()
	if m.NeedsRefreshAny() {
		fmt.Fprintln(os.Stderr, "lendo as cotas…")
		warns, err := m.RefreshAll()
		if err != nil {
			die("%v", err)
		}
		for _, w := range warns {
			fmt.Fprintln(os.Stderr, "aviso:", w)
		}
	}
	best := m.Best(cfg)
	if best == "" {
		die("nenhuma conta com cota agora — veja: mgy ls")
	}
	if err := m.SwitchTo(best); err != nil {
		die("%v", err)
	}
	fmt.Fprintf(os.Stderr, "◆ melhor conta: %s (cota %d%%)\n", best, cfg.Score(m.LoadQuota(best), time.Now()))
	launch(m, args)
}

func cmdConfig(m *core.Mgr, args []string) {
	cfg := m.LoadConfig()
	if len(args) == 0 {
		onoff := func(b bool) string {
			if b {
				return "on"
			}
			return "off"
		}
		fmt.Printf("strategy          %s\nfamily            %s\ncontinue          %s\nskip-permissions  %s\n",
			cfg.Strategy, cfg.Family, onoff(cfg.Continue), onoff(cfg.SkipPermissions))
		return
	}
	if len(args) != 2 {
		die("uso: mgy config <chave> <valor>")
	}
	if err := cfg.Set(args[0], args[1]); err != nil {
		die("%v", err)
	}
	if err := m.SaveConfig(cfg); err != nil {
		die("%v", err)
	}
	fmt.Println("ok:", args[0], "=", args[1])
}
