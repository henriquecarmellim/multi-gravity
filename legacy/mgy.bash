#!/usr/bin/env bash
# mgy — contas do Antigravity CLI (agy): ver cotas e trocar de conta pelo TUI.
#
#   mgy                abre o TUI (fzf): escolha a conta -> troca -> abre o agy
#   mgy -- <args>      igual, repassando <args> ao agy (ex.: mgy -- -c)
#   mgy add            guarda o login atual do agy como uma conta do mgy
#   mgy ls             lista as contas e as cotas (do cache)
#   mgy quota          atualiza a cota de todas as contas e lista
#   mgy use <email|n>  troca de conta e abre o agy
#   mgy rm <email>     esquece uma conta guardada
#
# Como funciona: o agy lê o login de ~/.gemini/antigravity-cli/antigravity-oauth-token.
# O mgy guarda uma cópia desse arquivo por conta em ~/.local/share/mgy/accounts/
# (modo 600, fora de qualquer repo) e troca copiando a escolhida de volta.
# A cota vem do próprio agy: `agy -p "/quota"`.
#
# Para cadastrar cada conta: no agy faça /logout e login com a conta, depois `mgy add`.

set -uo pipefail
umask 077

LIVE="$HOME/.gemini/antigravity-cli/antigravity-oauth-token"
STORE="${XDG_DATA_HOME:-$HOME/.local/share}/mgy"
ACC="$STORE/accounts"
QD="$STORE/quota"
SELF="$(readlink -f "${BASH_SOURCE[0]}")"

R=$'\e[38;2;255;45;57m'; P2=$'\e[38;2;255;91;100m'; P3=$'\e[38;2;255;133;140m'
D=$'\e[2m'; B=$'\e[1m'; Z=$'\e[0m'

die()  { printf '%smgy:%s %s\n' "$R" "$Z" "$*" >&2; exit 1; }
info() { printf '%s%s%s\n' "$P3" "$*" "$Z" >&2; }
need() { command -v "$1" >/dev/null || die "preciso do '$1' instalado"; }

mkdir -p "$ACC" "$QD"; chmod 700 "$STORE" "$ACC" "$QD"

lock() {
    exec 9>"$STORE/.lock"
    flock -n 9 || die "outra instância do mgy está rodando"
}

email_of() { jq -r '.id_token_email // empty' "$1" 2>/dev/null; }
valid_email() { [[ "$1" =~ ^[A-Za-z0-9._%+@-]+$ && "$1" != *..* ]]; }
accounts() { local f; for f in "$ACC"/*.json; do [[ -e "$f" ]] && basename "${f%.json}"; done; }
agy_running() { pgrep -x agy >/dev/null; }

# Grava o login vivo de volta no slot da conta (o agy pode renovar o token).
save_live() {
    local e; e=$(email_of "$LIVE")
    [[ -n "$e" && -f "$ACC/$e.json" ]] && cp -f "$LIVE" "$ACC/$e.json"
    return 0
}

# Coloca o login da conta no lugar do agy (cópia atômica, nunca symlink).
install_acc() {
    local tmp="$LIVE.mgy.$$"
    cp -f "$ACC/$1.json" "$tmp" && chmod 600 "$tmp" && mv -f "$tmp" "$LIVE"
}

# A troca só é segura se o login atual não for perdido e o agy estiver fechado.
can_swap() {
    agy_running && { info "agy está aberto — feche-o antes de trocar de conta"; return 1; }
    if [[ -f "$LIVE" ]]; then
        local e; e=$(email_of "$LIVE")
        if [[ -n "$e" && ! -f "$ACC/$e.json" ]]; then
            info "o login atual ($e) não está guardado — rode 'mgy add' antes"
            return 1
        fi
    fi
    return 0
}

switch_to() {
    local e="$1"
    [[ -f "$ACC/$e.json" ]] || die "conta '$e' não cadastrada (veja: mgy ls)"
    [[ "$(email_of "$LIVE")" == "$e" ]] && return 0
    can_swap || return 1
    save_live
    install_acc "$e" || die "falha ao copiar o login"
}

# ── cota ─────────────────────────────────────────────────────────────────────
# Lê a cota da conta que está ativa no agy e guarda em cache.
fetch_quota() {
    local out
    out=$(timeout 60 agy -p "/quota" 9>&- 2>/dev/null | grep -P '^[^\t]+\t[^\t]+\t') || return 1
    [[ -n "$out" ]] || return 1
    printf '%s\n' "$out" >"$QD/.$1.$$" && mv -f "$QD/.$1.$$" "$QD/$1.tsv"
}

refresh_active() {
    local e; e=$(email_of "$LIVE"); [[ -n "$e" ]] || return 0
    local f="$QD/$e.tsv"
    # cache com menos de 5 min vale
    [[ -f "$f" && $(( $(date +%s) - $(stat -c %Y "$f") )) -lt 300 ]] && return 0
    fetch_quota "$e" || true
}

ORIG=""
restore_orig() {
    [[ -n "$ORIG" && -f "$ACC/$ORIG.json" ]] && install_acc "$ORIG"
}

refresh_all() {
    lock
    need agy
    local orig e; orig=$(email_of "$LIVE")
    if can_swap; then
        save_live
        ORIG="$orig"
        trap restore_orig EXIT
        trap 'exit 130' INT TERM
        [[ -n "$orig" ]] && { fetch_quota "$orig" || info "sem cota para $orig"; }
        while read -r e; do
            [[ "$e" == "$orig" ]] && continue
            install_acc "$e" && { fetch_quota "$e" || info "sem cota para $e (login expirado?)"; save_live; }
        done < <(accounts)
        restore_orig; trap - EXIT
    else
        [[ -n "$orig" ]] && fetch_quota "$orig"   # só a ativa
    fi
}

# "6%" colorido; acima de 50% normal, abaixo rosa, 10% ou menos vermelho forte.
pct_cell() {
    local v="$1" n c
    if [[ "$v" == *% ]]; then
        n=${v%\%}; c=$Z; (( n <= 50 )) && c=$P3; (( n <= 10 )) && c=$R$B
        printf '%s%4s%s' "$c" "$v" "$Z"
    else printf '%s%4s%s' "$D" "—" "$Z"; fi
}

reset_fmt() {
    [[ -n "$1" ]] || { printf '%s' "     "; return; }
    local t; t=$(date -d "$1" +%s 2>/dev/null) || { printf '?'; return; }
    if [[ $(date -d "@$t" +%F) == "$(date +%F)" ]]; then date -d "@$t" +'%H:%M'; else date -d "@$t" +'%d/%m'; fi
}

age_fmt() {
    local f="$1"; [[ -f "$f" ]] || { printf 'sem dados'; return; }
    local s=$(( $(date +%s) - $(stat -c %Y "$f") ))
    if   (( s < 90 ));    then printf 'agora'
    elif (( s < 3600 ));  then printf 'há %dmin' $((s/60))
    elif (( s < 86400 )); then printf 'há %dh' $((s/3600))
    else printf 'há %dd' $((s/86400)); fi
}

# Uma linha por conta: "<email>\t<texto colorido>"
list_rows() {
    local live e f g l p r cols mark
    live=$(email_of "$LIVE")
    while read -r e; do
        f="$QD/$e.tsv"; cols=""
        if [[ -f "$f" ]]; then
            while IFS=$'\t' read -r g l p r; do
                g=${g/Claude and GPT models/Claude/GPT}; g=${g/ Models/}
                l=${l/ Limit Remaining/}; l=${l/Five Hour/5h}; l=${l/Weekly/sem}
                cols+=$(printf '%s%-10s%s %s %s%-5s%s  ' "$D" "$g $l" "$Z" "$(pct_cell "$p")" "$D" "$(reset_fmt "$r")" "$Z")
            done <"$f"
        else cols="${D}(sem dados — ctrl-r atualiza)${Z}"; fi
        if [[ "$e" == "$live" ]]; then mark="${R}●${Z}"; else mark="${D}○${Z}"; fi
        printf '%s\t%s %-30s %s %s%s%s\n' "$e" "$mark" "$e" "$cols" "$D" "$(age_fmt "$f")" "$Z"
    done < <(accounts)
}

preview() {
    local e="$1" f="$QD/$1.tsv" g l p r
    printf '%s%s%s  %s\n' "$B" "$e" "$Z" "$([[ "$(email_of "$LIVE")" == "$e" ]] && echo "${R}(ativa no agy)${Z}")"
    [[ -f "$f" ]] || { echo "sem dados de cota ainda"; return; }
    while IFS=$'\t' read -r g l p r; do
        printf '  %-22s %-26s %s  %s\n' "$g" "$l" "$(pct_cell "$p")" "${r:+renova $(date -d "$r" +'%d/%m %H:%M')}"
    done <"$f"
}

# ── comandos ─────────────────────────────────────────────────────────────────
# Guarda o login atual do agy como conta. Imprime o e-mail; falha se não houver login válido.
register_live() {
    [[ -f "$LIVE" ]] || return 1
    local e; e=$(email_of "$LIVE")
    [[ -n "$e" ]] && valid_email "$e" || return 1
    cp -f "$LIVE" "$ACC/$e.json"; chmod 600 "$ACC/$e.json"
    printf '%s' "$e"
}

cmd_add() {
    lock; need jq
    local e; e=$(register_live) || die "nenhum login válido no agy — abra o agy e faça login primeiro"
    fetch_quota "$e" || true
    printf '%s✓%s conta guardada: %s  (%d no total)\n' "$R" "$Z" "$e" "$(accounts | wc -l)"
}

# Adicionar conta pelo TUI: guarda o login atual, abre o agy deslogado para você
# entrar com a conta nova, registra e volta ao TUI. Sem login novo, restaura o antigo.
add_flow() {
    lock; need agy; need jq
    agy_running && { info "agy está aberto — feche-o antes de adicionar uma conta"; read -rsn1 -p "(enter)"; exec "$SELF" 9>&-; }
    local orig new
    if [[ -f "$LIVE" ]]; then
        orig=$(register_live) || orig=""        # garante que o login atual não se perde
        [[ -z "$orig" ]] && mv -f "$LIVE" "$STORE/.login-anterior.bak"
    fi
    rm -f "$LIVE"
    printf '\n%s%smgy — adicionar conta%s\n' "$R" "$B" "$Z"
    printf '%s1.%s o agy vai abrir pedindo login: entre com a conta NOVA no navegador\n' "$P3" "$Z"
    printf '%s2.%s depois saia do agy (/exit ou ctrl-c) para o mgy guardar a conta\n\n' "$P3" "$Z"
    read -rsn1 -p "enter para abrir o agy (ctrl-c cancela)… " ; echo
    agy 9>&-
    new=""
    if [[ -f "$LIVE" ]]; then new=$(register_live) || new=""; fi
    if [[ -n "$new" ]]; then
        fetch_quota "$new" || true
        if [[ "$new" == "$orig" ]]; then info "é a mesma conta de antes ($new) — atualizada"
        else printf '%s✓%s conta adicionada: %s\n' "$R" "$Z" "$new"; fi
    else
        [[ -n "$orig" ]] && install_acc "$orig"
        info "nenhum login novo — conta anterior restaurada"
    fi
    sleep 1
    exec "$SELF" 9>&-
}

# Remover pelo TUI (pede confirmação)
rm_confirm() {
    local e="$1"
    printf '\n%sRemover %s%s%s do mgy?%s (o login só é esquecido aqui) [s/N] ' "$P3" "$B" "$e" "$Z$P3" "$Z"
    read -rn1 a; echo
    [[ "$a" =~ ^[sS]$ ]] && cmd_rm "$e"
    return 0
}

cmd_rm() {
    lock
    valid_email "${1:-}" || die "uso: mgy rm <email>"
    [[ -f "$ACC/$1.json" ]] || die "conta '$1' não existe"
    rm -f "$ACC/$1.json" "$QD/$1.tsv"
    printf '%s✓%s removida: %s\n' "$R" "$Z" "$1"
}

resolve() {  # número da lista ou e-mail -> e-mail
    if [[ "$1" =~ ^[0-9]+$ ]]; then accounts | sed -n "${1}p"; else printf '%s' "$1"; fi
}

cmd_use() {
    lock; need agy
    local e; e=$(resolve "${1:-}"); [[ -n "$e" ]] || die "uso: mgy use <email|n>"
    shift
    switch_to "$e" || exit 1
    exec agy "$@" 9>&-
}

cmd_ls() {
    [[ -n "$(accounts)" ]] || die "nenhuma conta guardada — entre no agy e rode: mgy add"
    local n=0 line
    while IFS= read -r line; do n=$((n+1)); printf '%s%d%s %s\n' "$D" "$n" "$Z" "${line#*$'\t'}"; done < <(list_rows)
}

cmd_tui() {
    need fzf; need jq; need agy
    # o login que já está no agy entra sozinho na lista
    local cur; cur=$(email_of "$LIVE")
    [[ -n "$cur" && ! -f "$ACC/$cur.json" ]] && register_live >/dev/null
    [[ -n "$(accounts)" ]] || { info "nenhuma conta ainda — vamos adicionar a primeira"; add_flow; }
    refresh_active
    local sel e
    sel=$(list_rows | fzf --ansi --delimiter=$'\t' --with-nth=2 --no-sort --reverse \
        --height=~70% --border=rounded --info=inline-right --prompt='mgy ❯ ' --pointer='▶' \
        --header=$'↵ trocar e abrir o agy   ctrl-a adicionar conta   ctrl-x remover   ctrl-r atualizar cotas   esc sair\n' \
        --bind "ctrl-a:become($SELF _add_flow)" \
        --bind "ctrl-x:execute($SELF _rm_confirm {1})+reload($SELF _rows)" \
        --color='bg:-1,bg+:#1c1c22,fg+:#ffffff,hl:#ff5b64,hl+:#ff5b64,pointer:#ff2d39,prompt:#ff2d39,border:#ff2d39,header:#ff858c,info:#ff858c,gutter:-1' \
        --bind "ctrl-r:reload($SELF _refresh_all >/dev/null 2>&1; $SELF _rows)" \
        --preview "$SELF _preview {1}" --preview-window=down,7,border-top) || exit 0
    e=${sel%%$'\t'*}
    lock
    switch_to "$e" || exit 1
    printf '%s▶%s %s\n' "$R" "$Z" "$e" >&2
    exec agy "$@" 9>&-
}

case "${1:-}" in
    add)           cmd_add ;;
    rm|remove)     shift; cmd_rm "$@" ;;
    ls|list)       cmd_ls ;;
    quota|q)       refresh_all; cmd_ls ;;
    use)           shift; cmd_use "$@" ;;
    _rows)         list_rows ;;
    _add_flow)     add_flow ;;
    _rm_confirm)   rm_confirm "$2" ;;
    _preview)      preview "$2" ;;
    _refresh_all)  refresh_all ;;
    -h|--help|help) sed -n '2,15p' "$SELF" | sed 's/^# \{0,1\}//' ;;
    --)            shift; cmd_tui "$@" ;;
    '')            cmd_tui ;;
    *)             die "comando desconhecido: $1 (veja: mgy help)" ;;
esac
