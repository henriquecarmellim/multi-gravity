# ◆ multi-gravity

**Várias contas do [Antigravity CLI](https://antigravity.google/) (`agy`) num lugar só:** veja a cota de cada
uma e troque de conta por um TUI. O comando é **`mgy`**.

```
 ◆ multi-gravity  contas do agy                                                                                  3 conta(s)

                       Gemini 5h            Gemini sem           Claude/GPT 5h        Claude/GPT sem
 ▶ ● ana@exemplo.com    ████████ 100% ↻13:58  █████░░░  64% ↻29/11  desativado           ██████░░  80% ↻11:58
   ○ bruno@exemplo.com  ████████ 100% ↻13:58  ░░░░░░░░   6% ↻29/11  desativado           ░░░░░░░░   0% ↻11:58
   ○ carla@exemplo.com  ███░░░░░  35% ↻13:58  ███████░  92% ↻29/11  desativado           ██████░░  71% ↻11:58

  ╭────────────────────────────────────────────────────────╮
  │ ana@exemplo.com  (ativa no agy)   cota lida agora      │
  │ Gemini 5h       ████████████ 100%   renova 25/11 13:58 │
  │ Gemini sem      ████████░░░░  64%   renova 29/11 10:58 │
  │ Claude/GPT 5h   desativado                             │
  │ Claude/GPT sem  ██████████░░  80%   renova 25/11 11:58 │
  ╰────────────────────────────────────────────────────────╯
 ↑↓ mover  ·  ↵ trocar e abrir o agy  ·  a adicionar  ·  x remover  ·  r atualizar cotas  ·  q sair
```

*(exemplo com contas fictícias)*

- **Todas as cotas de uma vez:** Gemini e Claude/GPT, janela de 5 horas e semanal, com barra e horário de renovação.
- **Tudo pelo TUI:** trocar, adicionar, remover e atualizar as cotas, sem decorar comandos.
- **Troca e já abre o `agy`:** `Enter` na conta escolhida e você cai direto no `agy` com ela.
- **Linux, macOS e Windows**, um único binário, sem dependências em tempo de execução.
- **Seguro:** os logins ficam só na sua máquina, com permissão restrita, e nunca aparecem na tela.

## Instalação

Precisa do `agy` instalado. Para compilar, precisa também do [Go](https://go.dev/dl/).

**Linux e macOS, sem clonar** (baixa o binário da última release):

```bash
curl -fsSL https://raw.githubusercontent.com/henriquecarmellim/multi-gravity/main/install.sh | bash
```

**Linux e macOS, a partir do código:**

```bash
git clone https://github.com/henriquecarmellim/multi-gravity.git
cd multi-gravity
./install.sh              # instala em ~/.local/bin (use --prefix DIR para outro lugar)
```

**Windows (PowerShell), sem clonar** *(o caminho do Windows só foi validado pelo CI, não numa máquina real)*:

```powershell
irm https://raw.githubusercontent.com/henriquecarmellim/multi-gravity/main/install.ps1 | iex
```

**Windows, a partir do código:**

```powershell
git clone https://github.com/henriquecarmellim/multi-gravity.git
cd multi-gravity
.\install.ps1             # instala em %LOCALAPPDATA%\Programs\mgy e põe no PATH
```

Os scripts compilam dentro do clone; fora dele, baixam o binário da última
[release](https://github.com/henriquecarmellim/multi-gravity/releases) (e, se não houver release, o `install.sh`
clona e compila). Para desinstalar: `./install.sh --uninstall` ou `.\install.ps1 -Uninstall`
(as contas guardadas não são apagadas).

Também dá para compilar à mão: `go build -o mgy .`

> `~/.local/bin` (ou a pasta que você escolheu) precisa estar no `PATH`. O instalador avisa se não estiver.

## Como usar

Digite **`mgy`**. Pronto, o TUI abre.

| Tecla | O que faz |
|---|---|
| `↑` `↓` ou `j` `k` | mover entre as contas |
| `Enter` | trocar para a conta e abrir o `agy` |
| `a` | adicionar uma conta |
| `x` | remover a conta selecionada (pede confirmação) |
| `r` | atualizar a cota de todas as contas |
| `q` ou `Esc` | sair |

`●` marca a conta que está ativa no `agy` agora e `○` as demais. Valores baixos ficam em rosa (50% ou menos)
e em vermelho forte (10% ou menos). `↻` mostra quando a cota renova.

### Adicionando as contas

1. Rode `mgy`. A conta que já está logada no `agy` entra na lista sozinha.
2. Aperte **`a`**. O `mgy` guarda o login atual, limpa o login do `agy` e abre o `agy` pedindo login.
3. Entre com a conta **nova** no navegador e saia do `agy` (`/exit` ou `ctrl+c`).
4. O `mgy` guarda a conta, lê a cota e volta para a lista. Repita para cada conta.

Se você sair do `agy` sem fazer login, o login anterior é restaurado: nada se perde.

### Sem o TUI

Tudo também funciona por comando, bom para scripts:

| Comando | O que faz |
|---|---|
| `mgy` | abre o TUI |
| `mgy -- <args>` | abre o TUI e repassa `<args>` ao `agy` (ex.: `mgy -- -c` continua a última conversa) |
| `mgy ls` | lista as contas e as cotas (do cache) |
| `mgy quota` | atualiza a cota de todas as contas e lista |
| `mgy add` | adiciona uma conta (abre o `agy` para o login) |
| `mgy use <n\|email> [args]` | troca de conta e abre o `agy` |
| `mgy rm <email>` | esquece uma conta |
| `mgy version` | mostra a versão |

## Como funciona

O `agy` guarda o login num arquivo, `antigravity-oauth-token`. O mgy mantém **uma cópia desse arquivo por conta**
e troca de conta copiando a escolhida de volta (por `rename`, de forma atômica). Antes de trocar, grava o
login atual de volta no slot dele, porque o `agy` pode renovar o token.

A cota vem do próprio `agy`, pelo comando `agy -p "/quota"`. Para ler a cota de uma conta que não está ativa,
o mgy troca o login por um instante, lê e devolve o original. Por isso a cota das contas inativas só é
atualizada quando você aperta `r` (ou roda `mgy quota`). A conta ativa é lida ao abrir o TUI.

| | Linux / macOS | Windows |
|---|---|---|
| Login do `agy` | `~/.gemini/antigravity-cli/antigravity-oauth-token` | `%USERPROFILE%\.gemini\antigravity-cli\antigravity-oauth-token` ⚠️ |
| Contas e cotas do mgy | `~/.local/share/mgy/` (`$XDG_DATA_HOME/mgy`) | `%LOCALAPPDATA%\mgy\` |

⚠️ No Windows o caminho do login foi assumido igual ao do Linux e **não foi verificado**. Se o seu for outro,
defina `MGY_LIVE` com o caminho certo.

### ⚠️ Limitação: cofre de senhas do sistema

O `agy` tenta ler o login primeiro do **cofre de senhas do sistema** (Secret Service no Linux, Keychain no macOS,
Credential Manager no Windows) e só depois do arquivo. O mgy troca **o arquivo**. Onde o `agy` guarda o login
no cofre em vez do arquivo, a troca não terá efeito, e o mgy mostrará "nenhum login" ou contas erradas.

Foi testado num Linux sem cofre ativo, onde o `agy` usa o arquivo (e, sem ele, fica deslogado e pede login). Em macOS
e Windows isso **não foi verificado**. Se o mgy não trocar a conta no seu sistema, abra uma issue com a saída de
`mgy ls` e o sistema que você usa.

### Variáveis de ambiente

| Variável | Para quê |
|---|---|
| `MGY_LIVE` | arquivo de login do `agy` |
| `MGY_STORE` | pasta onde o mgy guarda contas e cotas |
| `MGY_AGY` | caminho do binário do `agy` |

## Segurança

- Os logins ficam em `accounts/`, com pasta `700` e arquivos `600`, **fora de qualquer repositório**. O `.gitignore`
  deste projeto bloqueia `*oauth-token*` e `accounts/`.
- Nenhum token é mostrado na tela, nem nos logs, nem nas mensagens de erro.
- O mgy **recusa trocar de conta com o `agy` aberto** e **nunca troca se o login atual não estiver guardado**,
  para não perder uma conta.
- Se for interrompido no meio de uma atualização de cotas (`ctrl+c`, fechar o terminal), o login original é restaurado.
- Duas instâncias do mgy não mexem no login ao mesmo tempo (trava por arquivo).

Quem tiver acesso a `accounts/` tem acesso às suas contas. Trate essa pasta como uma senha.

## Problemas comuns

| Mensagem | O que fazer |
|---|---|
| `o agy está aberto — feche-o antes de trocar de conta` | feche todas as sessões do `agy` (inclusive o daemon `agy remote-control`, se estiver rodando) |
| `o login atual (...) não está guardado` | abra o `mgy` e aperte `a`, ou rode `mgy add`: assim o login atual é guardado antes de qualquer troca |
| cota `sem dados` / `login expirado?` | o login dessa conta expirou: troque para ela, faça login de novo no `agy` e adicione a conta outra vez |
| `não achei o agy` | instale o `agy` ou aponte `MGY_AGY` |

## Desenvolvimento

```bash
make test        # go vet + go test (o CI roda em Linux, macOS e Windows)
make build       # gera ./mgy
make dist        # binários de todos os sistemas em dist/ (precisa de zip)
```

O código fica em `internal/core` (contas, troca de login e cota) e `internal/tui` (a tela, com
[Bubble Tea](https://github.com/charmbracelet/bubbletea)). Os testes usam um `agy` de mentira e pastas
temporárias: nunca tocam no seu login real. Para publicar uma versão: `git tag v0.1.0 && git push --tags`
(o workflow de release gera os binários).

## Aviso

Projeto independente, sem ligação com o Google. Ele só copia arquivos de login do `agy` e chama o comando
`/quota` que o próprio `agy` oferece; como o `agy` guarda o login num arquivo interno que pode mudar entre
versões, uma atualização dele pode quebrar o mgy. Use apenas contas suas e respeite os termos de uso do serviço.

## Licença

[MIT](LICENSE)
