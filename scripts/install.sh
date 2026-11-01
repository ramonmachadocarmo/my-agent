#!/bin/sh
# Installs my-agent on Linux and macOS and sets up the "my-agent" command in
# bash, zsh, fish, sh and PowerShell (pwsh).
#
#   sh scripts/install.sh                 # install for the current user (~/.local/bin)
#   sh scripts/install.sh --system        # install to /usr/local/bin (asks for sudo)
#   sh scripts/install.sh --uninstall     # remove (add --purge to delete config and sessions)
#
# It uses the my-agent binary next to this script (release archive), or
# builds it from source when run inside the repository with Go installed, or
# uses bin/my-agent. Run it again to update.
set -eu

BIN_DIR="$HOME/.local/bin"
BINARY=""
UNINSTALL=0
PURGE=0
NO_SHELL=0
NO_BUILD=0
SUDO=""

# ---------------------------------------------------------------- messages --
case "${MY_AGENT_LANG:-${LC_ALL:-${LC_MESSAGES:-${LANG:-en}}}}" in
  pt*) L=pt ;; es*) L=es ;; *) L=en ;;
esac
msg() {
  key=$1; shift
  case "$L:$key" in
    en:usage)     t='usage: install.sh [--system | --bin-dir DIR] [--binary FILE] [--no-shell] [--no-build] [--uninstall [--purge]]' ;;
    pt:usage)     t='uso: install.sh [--system | --bin-dir PASTA] [--binary ARQUIVO] [--no-shell] [--no-build] [--uninstall [--purge]]' ;;
    es:usage)     t='uso: install.sh [--system | --bin-dir CARPETA] [--binary ARCHIVO] [--no-shell] [--no-build] [--uninstall [--purge]]' ;;
    en:windows)   t='On Windows use scripts\install.ps1 (or install.cmd).' ;;
    pt:windows)   t='No Windows use scripts\install.ps1 (ou install.cmd).' ;;
    es:windows)   t='En Windows usa scripts\install.ps1 (o install.cmd).' ;;
    en:building)  t='Building my-agent from source...' ;;
    pt:building)  t='Compilando o my-agent a partir do código-fonte...' ;;
    es:building)  t='Compilando my-agent desde el código fuente...' ;;
    en:nobinary)  t='my-agent binary not found. Run this script from a release archive, or from the repository with Go installed (https://go.dev/dl).' ;;
    pt:nobinary)  t='binário do my-agent não encontrado. Rode este script a partir do pacote de release, ou no repositório com o Go instalado (https://go.dev/dl).' ;;
    es:nobinary)  t='no se encontró el binario de my-agent. Ejecuta este script desde el paquete de release, o en el repositorio con Go instalado (https://go.dev/dl).' ;;
    en:badbinary) t='%s does not run on this system (wrong OS or architecture?).' ;;
    pt:badbinary) t='%s não roda neste sistema (sistema ou arquitetura errados?).' ;;
    es:badbinary) t='%s no funciona en este sistema (¿sistema o arquitectura incorrectos?).' ;;
    en:copying)   t='Installing to %s' ;;
    pt:copying)   t='Instalando em %s' ;;
    es:copying)   t='Instalando en %s' ;;
    en:profile)   t='Configured: %s' ;;
    pt:profile)   t='Configurado: %s' ;;
    es:profile)   t='Configurado: %s' ;;
    en:unprofile) t='Removed from: %s' ;;
    pt:unprofile) t='Removido de: %s' ;;
    es:unprofile) t='Eliminado de: %s' ;;
    en:done)      t='my-agent %s installed.' ;;
    pt:done)      t='my-agent %s instalado.' ;;
    es:done)      t='my-agent %s instalado.' ;;
    en:next)      t='Open a NEW terminal (or run: %s) and type:  my-agent' ;;
    pt:next)      t='Abra um NOVO terminal (ou rode: %s) e digite:  my-agent' ;;
    es:next)      t='Abre una NUEVA terminal (o ejecuta: %s) y escribe:  my-agent' ;;
    en:removed)   t='my-agent removed.' ;;
    pt:removed)   t='my-agent removido.' ;;
    es:removed)   t='my-agent eliminado.' ;;
    en:kept)      t='Your configuration and sessions were kept: %s, %s (use --purge to delete them).' ;;
    pt:kept)      t='Sua configuração e sessões foram mantidas: %s, %s (use --purge para apagá-las).' ;;
    es:kept)      t='Se conservaron tu configuración y sesiones: %s, %s (usa --purge para borrarlas).' ;;
    en:purged)    t='Deleted: %s' ;;
    pt:purged)    t='Apagado: %s' ;;
    es:purged)    t='Borrado: %s' ;;
  esac
  # shellcheck disable=SC2059
  printf "$t\n" "$@"
}

if [ -t 1 ]; then G='\033[32m'; C='\033[36m'; R='\033[31m'; D='\033[2m'; N='\033[0m'; else G=; C=; R=; D=; N=; fi
say()  { printf '%b%s%b\n' "$2" "$1" "$N"; }

# ------------------------------------------------------------------- args --
while [ $# -gt 0 ]; do
  case "$1" in
    --system)    BIN_DIR=/usr/local/bin ;;
    --bin-dir)   BIN_DIR=$2; shift ;;
    --bin-dir=*) BIN_DIR=${1#*=} ;;
    --binary)    BINARY=$2; shift ;;
    --binary=*)  BINARY=${1#*=} ;;
    --uninstall) UNINSTALL=1 ;;
    --purge)     PURGE=1 ;;
    --no-shell)  NO_SHELL=1 ;;
    --no-build)  NO_BUILD=1 ;;
    -h|--help)   msg usage; exit 0 ;;
    *)           msg usage >&2; exit 2 ;;
  esac
  shift
done

OS=$(uname -s)
case "$OS" in
  MINGW*|MSYS*|CYGWIN*) msg windows >&2; exit 2 ;;
esac

# a folder the user cannot write to needs sudo
mkdir -p "$BIN_DIR" 2>/dev/null || true
if [ ! -w "$BIN_DIR" ] 2>/dev/null || [ ! -d "$BIN_DIR" ]; then
  if [ "$(id -u)" -ne 0 ]; then SUDO=sudo; fi
fi
DEST="$BIN_DIR/my-agent"

BEGIN='# >>> my-agent >>>'
END='# <<< my-agent <<<'

# remove_block FILE: deletes the lines between the markers (keeps the file's
# permissions; works with the BSD tools of macOS).
remove_block() {
  f=$1
  [ -f "$f" ] || return 1
  grep -qF "$BEGIN" "$f" || return 1
  tmp=$(mktemp)
  awk -v b="$BEGIN" -v e="$END" '$0==b{skip=1} !skip{print} $0==e{skip=0}' "$f" > "$tmp"
  cat "$tmp" > "$f"
  rm -f "$tmp"
  return 0
}

# set_block FILE CONTENT: writes CONTENT between the markers, replacing an
# older block.
set_block() {
  f=$1
  mkdir -p "$(dirname "$f")"
  [ -f "$f" ] || : > "$f"
  remove_block "$f" || true
  # keep one blank line before the block
  if [ -s "$f" ] && [ -n "$(tail -c 1 "$f")" ]; then printf '\n' >> "$f"; fi
  printf '%s\n%s\n%s\n' "$BEGIN" "$2" "$END" >> "$f"
  say "$(msg profile "$f")" "$G"
}

path_line() {
  printf 'case ":$PATH:" in *":%s:"*) ;; *) export PATH="%s:$PATH" ;; esac' "$BIN_DIR" "$BIN_DIR"
}

PWSH_PROFILE="$HOME/.config/powershell/profile.ps1"
FISH_CONF="${XDG_CONFIG_HOME:-$HOME/.config}/fish/conf.d/my-agent.fish"
PROFILES="$HOME/.bashrc $HOME/.bash_profile $HOME/.zshrc $HOME/.profile $PWSH_PROFILE"

# ============================================================== uninstall ==
if [ "$UNINSTALL" -eq 1 ]; then
  for d in "$HOME/.local/bin" /usr/local/bin "$BIN_DIR"; do
    if [ -e "$d/my-agent" ]; then
      if [ -w "$d" ]; then rm -f "$d/my-agent"; else sudo rm -f "$d/my-agent"; fi
    fi
  done
  for f in $PROFILES; do
    if remove_block "$f"; then say "$(msg unprofile "$f")" "$D"; fi
  done
  if [ -f "$FISH_CONF" ]; then rm -f "$FISH_CONF"; say "$(msg unprofile "$FISH_CONF")" "$D"; fi
  CFG="${XDG_CONFIG_HOME:-$HOME/.config}/my-agent"
  [ "$OS" = Darwin ] && CFG="$HOME/Library/Application Support/my-agent"
  DATA="${MY_AGENT_DATA:-${XDG_DATA_HOME:-$HOME/.local/share}/my-agent}"
  if [ "$PURGE" -eq 1 ]; then
    for d in "$CFG" "$DATA"; do
      if [ -e "$d" ]; then rm -rf "$d"; say "$(msg purged "$d")" "$D"; fi
    done
  else
    say "$(msg kept "$CFG" "$DATA")" "$D"
  fi
  say "$(msg removed)" "$G"
  exit 0
fi

# ================================================================ install ==
HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(dirname "$HERE")
SRC=""
if [ -n "$BINARY" ]; then
  SRC=$BINARY
elif [ -f "$HERE/my-agent" ]; then
  SRC="$HERE/my-agent"                                   # release archive
elif [ "$NO_BUILD" -eq 0 ] && [ -f "$ROOT/go.mod" ] && command -v go >/dev/null 2>&1; then
  say "$(msg building)" "$C"
  VERSION=$(sed -n 's/^## \[\([0-9][^]]*\)\].*/\1/p' "$ROOT/CHANGELOG.md" 2>/dev/null | head -n1)
  mkdir -p "$ROOT/bin"
  (cd "$ROOT" && go build -trimpath -ldflags "-s -w -X my-agent/internal/cli.Version=${VERSION:-dev}" -o bin/my-agent ./cmd/my-agent)
  SRC="$ROOT/bin/my-agent"
elif [ -f "$ROOT/bin/my-agent" ]; then
  SRC="$ROOT/bin/my-agent"
fi
if [ -z "$SRC" ]; then say "$(msg nobinary)" "$R" >&2; exit 1; fi
chmod +x "$SRC" 2>/dev/null || true
# macOS marks downloaded files; the quarantine flag would block the binary
[ "$OS" = Darwin ] && xattr -d com.apple.quarantine "$SRC" 2>/dev/null || true
if ! "$SRC" version >/dev/null 2>&1; then say "$(msg badbinary "$SRC")" "$R" >&2; exit 1; fi

say "$(msg copying "$BIN_DIR")" "$C"
$SUDO mkdir -p "$BIN_DIR"
# install to a temporary name, then rename: works while my-agent is running
$SUDO cp "$SRC" "$DEST.new"
$SUDO chmod 755 "$DEST.new"
$SUDO mv -f "$DEST.new" "$DEST"
[ "$OS" = Darwin ] && $SUDO xattr -d com.apple.quarantine "$DEST" 2>/dev/null || true

RELOAD=""
if [ "$NO_SHELL" -eq 0 ]; then
  PL=$(path_line)
  # bash: interactive shells read ~/.bashrc; macOS terminals open login
  # shells, which read ~/.bash_profile
  if command -v bash >/dev/null 2>&1; then
    set_block "$HOME/.bashrc" "$PL
command -v my-agent >/dev/null 2>&1 && eval \"\$(my-agent completion bash)\""
    if [ "$OS" = Darwin ] || [ -f "$HOME/.bash_profile" ]; then
      set_block "$HOME/.bash_profile" "$PL
[ -f ~/.bashrc ] && case \$- in *i*) . ~/.bashrc ;; esac"
    fi
    case "${SHELL:-}" in */bash) RELOAD=". ~/.bashrc" ;; esac
  fi
  # zsh (the default shell on macOS)
  if command -v zsh >/dev/null 2>&1 || [ -f "$HOME/.zshrc" ]; then
    set_block "$HOME/.zshrc" "$PL
command -v my-agent >/dev/null 2>&1 && eval \"\$(my-agent completion zsh)\""
    case "${SHELL:-}" in */zsh) RELOAD="source ~/.zshrc" ;; esac
  fi
  # sh, dash and graphical sessions read ~/.profile
  if [ "$OS" != Darwin ] || [ -f "$HOME/.profile" ]; then
    set_block "$HOME/.profile" "$PL"
  fi
  # fish
  if command -v fish >/dev/null 2>&1; then
    mkdir -p "$(dirname "$FISH_CONF")"
    printf '%s\n' "fish_add_path -g $BIN_DIR" "command -q my-agent; and my-agent completion fish | source" > "$FISH_CONF"
    say "$(msg profile "$FISH_CONF")" "$G"
    case "${SHELL:-}" in */fish) RELOAD="source $FISH_CONF" ;; esac
  fi
  # PowerShell on Linux/macOS
  if command -v pwsh >/dev/null 2>&1; then
    set_block "$PWSH_PROFILE" "if (-not ((\$env:PATH -split ':') -contains '$BIN_DIR')) { \$env:PATH = '$BIN_DIR:' + \$env:PATH }
if (Get-Command my-agent -ErrorAction SilentlyContinue) { my-agent completion powershell | Out-String | Invoke-Expression }"
  fi
fi

VER=$("$DEST" version | sed 's/^my-agent *//')
printf '\n'
say "$(msg done "$VER")" "$G"
say "$(msg next "${RELOAD:-exec \$SHELL -l}")" ""
