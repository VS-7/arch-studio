#!/bin/sh
# Instalador do CLI do ArchCode Studio para Linux e macOS.
#
#   curl -fsSL https://raw.githubusercontent.com/VS-7/arch-studio/main/scripts/install.sh | sh
#
# Baixa o binário da última release do GitHub para o sistema e a arquitetura
# atuais, confere o SHA-256 com o SHA256SUMS da release, instala em
# ~/.local/bin (sem sudo) e coloca essa pasta no PATH do seu shell.
# Rodar de novo atualiza para a versão mais recente.
#
# Opções (variáveis de ambiente):
#   ARCHCODE_VERSION=1.0.0          instala uma versão específica
#   ARCHCODE_INSTALL_DIR=/usr/local/bin   outra pasta de instalação
#   ARCHCODE_NO_MODIFY_PATH=1       não mexe nos arquivos de configuração do shell
#
# Exemplo: curl -fsSL …/install.sh | ARCHCODE_VERSION=1.0.0 sh

set -eu

REPO="VS-7/arch-studio"
BIN="archcode-studio"

say() { printf '%s\n' "$*"; }
fail() {
	printf 'erro: %s\n' "$*" >&2
	exit 1
}
has() { command -v "$1" >/dev/null 2>&1; }

# Sistema e arquitetura, nos nomes usados pelos arquivos da release.
detect_platform() {
	case "$(uname -s)" in
	Linux) os=linux ;;
	Darwin) os=darwin ;;
	*) fail "sistema não suportado: $(uname -s). No Windows, use o install.ps1 (veja o README)." ;;
	esac
	case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*) fail "arquitetura não suportada: $(uname -m)" ;;
	esac
	# Terminal sob Rosetta num Mac Apple Silicon: prefere o binário nativo.
	if [ "$os" = darwin ] && [ "$arch" = amd64 ] && [ "$(sysctl -n sysctl.proc_translated 2>/dev/null || true)" = 1 ]; then
		arch=arm64
	fi
}

fetch() {
	if has curl; then
		curl -fsSL --retry 3 -o "$2" "$1"
	elif has wget; then
		wget -q -O "$2" "$1"
	else
		fail "é preciso ter curl ou wget"
	fi
}

sha256() {
	if has sha256sum; then
		sha256sum "$1" | awk '{print $1}'
	elif has shasum; then
		shasum -a 256 "$1" | awk '{print $1}'
	else
		fail "é preciso ter sha256sum ou shasum para conferir o download"
	fi
}

# Garante a pasta no PATH das próximas sessões, no arquivo do shell do usuário.
add_to_path() {
	dir=$1
	case ":$PATH:" in *":$dir:"*) return 0 ;; esac
	if [ -n "${ARCHCODE_NO_MODIFY_PATH:-}" ]; then
		say "  Adicione $dir ao PATH para usar o comando $BIN."
		return 0
	fi
	line="export PATH=\"$dir:\$PATH\""
	case "$(basename "${SHELL:-sh}")" in
	zsh) rc="${ZDOTDIR:-$HOME}/.zshrc" ;;
	bash) if [ "$os" = darwin ]; then rc="$HOME/.bash_profile"; else rc="$HOME/.bashrc"; fi ;;
	fish)
		rc="$HOME/.config/fish/config.fish"
		line="fish_add_path \"$dir\""
		;;
	*) rc="$HOME/.profile" ;;
	esac
	mkdir -p "$(dirname "$rc")"
	if ! grep -qsF "$dir" "$rc"; then
		printf '\n# ArchCode Studio\n%s\n' "$line" >>"$rc"
	fi
	say "  PATH configurado em $rc."
	say "  Para usar neste terminal agora: export PATH=\"$dir:\$PATH\""
}

main() {
	detect_platform
	if [ -n "${ARCHCODE_VERSION:-}" ]; then
		base="https://github.com/$REPO/releases/download/v${ARCHCODE_VERSION#v}"
		label="v${ARCHCODE_VERSION#v}"
	else
		base="https://github.com/$REPO/releases/latest/download"
		label="mais recente"
	fi
	asset="$BIN-$os-$arch"
	dir="${ARCHCODE_INSTALL_DIR:-$HOME/.local/bin}"

	tmp=$(mktemp -d 2>/dev/null || mktemp -d -t archcode)
	trap 'rm -rf "$tmp"' EXIT INT TERM

	say "ArchCode Studio — instalando o CLI ($os/$arch, versão $label)"
	fetch "$base/$asset" "$tmp/$asset" || fail "não foi possível baixar $base/$asset"
	fetch "$base/SHA256SUMS" "$tmp/SHA256SUMS" || fail "não foi possível baixar $base/SHA256SUMS"

	expected=$(awk -v f="$asset" '$2 == f || $2 == "*" f {print $1}' "$tmp/SHA256SUMS")
	[ -n "$expected" ] || fail "$asset não consta no SHA256SUMS da release"
	actual=$(sha256 "$tmp/$asset")
	[ "$expected" = "$actual" ] || fail "o SHA-256 do download não confere (esperado $expected, obtido $actual)"
	say "  Download conferido (SHA-256)."

	mkdir -p "$dir" || fail "não foi possível criar $dir"
	chmod +x "$tmp/$asset"
	mv -f "$tmp/$asset" "$dir/$BIN" || fail "não foi possível gravar em $dir (tente ARCHCODE_INSTALL_DIR=\$HOME/.local/bin)"
	if [ "$os" = darwin ] && has xattr; then
		xattr -d com.apple.quarantine "$dir/$BIN" 2>/dev/null || true
	fi
	say "  Instalado em $dir/$BIN"
	add_to_path "$dir"

	say ""
	"$dir/$BIN" version
	say ""
	say "Pronto! Próximos passos:"
	say "  mkdir meu-projeto && cd meu-projeto"
	say "  $BIN init --name \"Meu Projeto\"     # cria .arch/, docs/ e api/"
	say "  $BIN serve                          # abre a interface em http://127.0.0.1:8765"
	say "  claude mcp add archcode-studio -- $BIN mcp --dir \"\$PWD\"   # conecta o Claude Code"
}

main "$@"
