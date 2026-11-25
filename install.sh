#!/usr/bin/env bash
# Instala o mgy (multi-gravity) em Linux e macOS.
#
#   ./install.sh                 instala em ~/.local/bin
#   ./install.sh --prefix DIR    instala em DIR
#   ./install.sh --uninstall     remove o binário (as contas guardadas ficam)
#
# Dentro de um clone, compila com Go. Fora dele, baixa o binário da última release
# do GitHub; se não houver, clona o repo e compila (precisa de Go e git).
set -euo pipefail

repo="henriquecarmellim/multi-gravity"
dest="$HOME/.local/bin"
uninstall=0

say()  { printf '\033[38;2;255;45;57m▶\033[0m %s\n' "$*"; }
warn() { printf '\033[38;2;255;133;140m!\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[38;2;255;45;57mx\033[0m %s\n' "$*" >&2; exit 1; }

while (( $# )); do
    case "$1" in
        --prefix)    dest="${2:?use: --prefix DIR}"; shift ;;
        --uninstall) uninstall=1 ;;
        -h|--help)   sed -n '2,10p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
        *)           die "opção desconhecida: $1" ;;
    esac
    shift
done

if (( uninstall )); then
    rm -f "$dest/mgy"
    say "mgy removido de $dest (as contas em ~/.local/share/mgy continuam)"
    exit 0
fi

case "$(uname -s)" in
    Linux)  os=linux ;;
    Darwin) os=darwin ;;
    *)      die "sistema não suportado por este script: $(uname -s) (no Windows use install.ps1)" ;;
esac
case "$(uname -m)" in
    x86_64|amd64)  arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *)             die "arquitetura não suportada: $(uname -m)" ;;
esac

mkdir -p "$dest"
tmp="$(mktemp -d)"
trap 'rm -rf -- "$tmp"' EXIT
here="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" 2>/dev/null && pwd || true)"

build() {  # $1 = pasta com o código
    command -v go >/dev/null || die "preciso do Go para compilar (https://go.dev/dl)"
    local v; v="$(git -C "$1" describe --tags --always --dirty 2>/dev/null || echo dev)"
    say "Compilando o mgy ($v)"
    (cd "$1" && go build -trimpath -ldflags "-s -w -X main.version=$v" -o "$tmp/mgy" .)
}

if [[ -n "$here" && -f "$here/go.mod" ]] && grep -q "multi-gravity" "$here/go.mod"; then
    build "$here"
else
    url="https://github.com/$repo/releases/latest/download/mgy_${os}_${arch}.tar.gz"
    say "Baixando $url"
    if curl -fsSL "$url" -o "$tmp/mgy.tar.gz" 2>/dev/null && tar -xzf "$tmp/mgy.tar.gz" -C "$tmp" mgy; then
        :
    else
        warn "sem release publicada — compilando do código-fonte"
        command -v git >/dev/null || die "preciso do git para clonar o repositório"
        git clone --depth 1 "https://github.com/$repo.git" "$tmp/src" >/dev/null 2>&1 || die "não consegui clonar $repo"
        build "$tmp/src"
    fi
fi

install -m 755 "$tmp/mgy" "$dest/mgy"
say "Instalado em $dest/mgy"

case ":$PATH:" in
    *":$dest:"*) ;;
    *) warn "$dest não está no PATH. Adicione ao seu shell:"
       warn "  bash/zsh: export PATH=\"$dest:\$PATH\""
       warn "  fish:     fish_add_path $dest" ;;
esac
command -v agy >/dev/null || warn "o agy (Antigravity CLI) não foi encontrado no PATH — instale-o ou aponte MGY_AGY"

say "Pronto! Rode:  mgy"
