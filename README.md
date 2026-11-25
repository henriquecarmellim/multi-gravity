# multi-gravity

Gerenciador de várias contas do **Antigravity CLI (`agy`)**: mostra a cota de cada
conta (Gemini e Claude/GPT, janelas de 5h e semanal) e troca de conta por um TUI.
O comando é `mgy`.

> Em desenvolvimento. A primeira versão (Bash + fzf) está em `legacy/mgy.bash`
> e serve de referência para a reescrita em Go + Bubble Tea.

## Como funciona

- O `agy` autentica pelo arquivo `~/.gemini/antigravity-cli/antigravity-oauth-token`.
- O mgy guarda uma cópia por conta em `~/.local/share/mgy/accounts/` (modo 600,
  fora de qualquer repo) e troca copiando a escolhida de volta.
- A cota vem do próprio agy: `agy -p "/quota"`.

## Segurança

Nenhum token é mostrado na tela nem versionado. O mgy recusa trocar de conta com o
`agy` aberto e nunca troca se o login atual não estiver guardado.

## Licença

MIT
