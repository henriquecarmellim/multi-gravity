// mgy — várias contas do Antigravity CLI (agy): cotas e troca por um TUI.
//
//	mgy                abre o TUI
//	mgy -- <args>      abre o TUI e repassa <args> ao agy
//	mgy ls             lista as contas e as cotas (do cache)
//	mgy quota          atualiza a cota de todas as contas e lista
//	mgy add            adiciona uma conta (abre o agy para o login)
//	mgy use <n|email>  troca de conta e abre o agy
//	mgy rm <email>     esquece uma conta
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/henriquecarmellim/multi-gravity/internal/core"
	"github.com/henriquecarmellim/multi-gravity/internal/tui"
)

var version = "dev"

const help = `mgy — várias contas do Antigravity CLI (agy)

uso:
  mgy                 abre o TUI (trocar, adicionar, remover e ver as cotas)
  mgy -- <args>       abre o TUI e repassa <args> ao agy (ex.: mgy -- -c)
  mgy ls              lista as contas e as cotas (do cache)
  mgy quota           atualiza a cota de todas as contas e lista
  mgy add             adiciona uma conta (abre o agy para você fazer login)
  mgy use <n|email>   troca de conta e abre o agy
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
		if a := m.Accounts(); n >= 1 && n <= len(a) {
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
	if err := launchAgy(m.Agy, args); err != nil {
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
	accts := m.Accounts()
	if len(accts) == 0 {
		die("nenhuma conta guardada — rode: mgy add")
	}
	live := m.ActiveEmail()
	for i, e := range accts {
		mark := "○"
		if e == live {
			mark = "●"
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
		fmt.Printf("%d %s %-32s %s\n", i+1, mark, e, strings.Join(cols, " | "))
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
