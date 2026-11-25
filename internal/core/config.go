package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Config são as opções do mgy (config.json dentro da pasta do mgy).
type Config struct {
	// Strategy ordena as contas: "best" (mais cota primeiro) ou "manual" (ordem alfabética).
	Strategy string `json:"strategy"`
	// Family diz qual cota pesa no "melhor": "gemini", "claude" (Claude + GPT) ou "both" (média).
	Family string `json:"family"`
	// Continue abre o agy com --continue, para a conversa seguir depois de trocar de conta.
	Continue bool `json:"continue"`
	// SkipPermissions abre o agy com --dangerously-skip-permissions.
	SkipPermissions bool `json:"skip_permissions"`
}

func DefaultConfig() Config {
	return Config{Strategy: "best", Family: "gemini", Continue: true, SkipPermissions: false}
}

var (
	Strategies = []string{"best", "manual"}
	Families   = []string{"gemini", "claude", "both"}
)

func (c Config) sane() Config {
	d := DefaultConfig()
	if !slices.Contains(Strategies, c.Strategy) {
		c.Strategy = d.Strategy
	}
	if !slices.Contains(Families, c.Family) {
		c.Family = d.Family
	}
	return c
}

func (m *Mgr) configFile() string { return filepath.Join(m.Store, "config.json") }

// LoadConfig lê as opções; arquivo ausente ou inválido vira o padrão.
func (m *Mgr) LoadConfig() Config {
	c := DefaultConfig()
	if b, err := os.ReadFile(m.configFile()); err == nil {
		_ = json.Unmarshal(b, &c) // campos ausentes ficam com o padrão
	}
	return c.sane()
}

func (m *Mgr) SaveConfig(c Config) error {
	if err := m.Init(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c.sane(), "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(m.Store, ".cfg-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err = tmp.Chmod(0o600); err == nil {
		_, err = tmp.Write(append(b, '\n'))
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, m.configFile())
}

// Set muda uma opção pelo nome (usado por `mgy config <chave> <valor>`).
func (c *Config) Set(key, val string) error {
	val = strings.ToLower(strings.TrimSpace(val))
	onoff := func() (bool, error) {
		switch val {
		case "on", "sim", "true", "1", "ligado":
			return true, nil
		case "off", "nao", "não", "false", "0", "desligado":
			return false, nil
		}
		return false, fmt.Errorf("use on ou off")
	}
	switch key {
	case "strategy":
		if !slices.Contains(Strategies, val) {
			return fmt.Errorf("strategy: use %s", strings.Join(Strategies, " | "))
		}
		c.Strategy = val
	case "family":
		if !slices.Contains(Families, val) {
			return fmt.Errorf("family: use %s", strings.Join(Families, " | "))
		}
		c.Family = val
	case "continue":
		v, err := onoff()
		if err != nil {
			return fmt.Errorf("continue: %w", err)
		}
		c.Continue = v
	case "skip-permissions":
		v, err := onoff()
		if err != nil {
			return fmt.Errorf("skip-permissions: %w", err)
		}
		c.SkipPermissions = v
	default:
		return fmt.Errorf("opção desconhecida: %s (strategy, family, continue, skip-permissions)", key)
	}
	return nil
}

// conversa/modo: se o usuário já escolheu, o mgy não mexe.
var conversationFlags = []string{"-c", "--continue", "--conversation", "-r", "--resume", "--new-project"}

// LaunchArgs monta os argumentos do agy: as opções do mgy e depois os do usuário.
func (c Config) LaunchArgs(extra []string) []string {
	var out []string
	if c.SkipPermissions && !slices.Contains(extra, "--dangerously-skip-permissions") {
		out = append(out, "--dangerously-skip-permissions")
	}
	if c.Continue {
		own := false
		for _, a := range extra {
			if slices.Contains(conversationFlags, a) {
				own = true
			}
		}
		if !own {
			out = append(out, "--continue")
		}
	}
	return append(out, extra...)
}
