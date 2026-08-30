#!/usr/bin/env bash
# Instala hels descargando el binario correcto desde el último release de GitHub.
#
# Uso:
#   curl -fsSL https://raw.githubusercontent.com/MoreCodeLess/hels-devexp/main/install.sh | bash
#
# Variables opcionales:
#   HELS_INSTALL_DIR   directorio destino (default: /usr/local/bin si es escribible, si no ~/.local/bin)
#   HELS_VERSION       tag a instalar, ej. v0.1.0 (default: el último release)
set -euo pipefail

REPO="MoreCodeLess/hels-devexp"
BIN_NAME="hels"

os="$(uname -s)"
case "$os" in
  Linux)  os="linux" ;;
  Darwin) os="darwin" ;;
  *)
    echo "hels todavía no soporta esta plataforma: $os (solo Linux y macOS)" >&2
    exit 1
    ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64)  arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *)
    echo "hels todavía no soporta esta arquitectura: $arch (solo amd64 y arm64)" >&2
    exit 1
    ;;
esac

version="${HELS_VERSION:-latest}"
asset="hels_${os}_${arch}.tar.gz"

if [ "$version" = "latest" ]; then
  url="https://github.com/${REPO}/releases/latest/download/${asset}"
else
  url="https://github.com/${REPO}/releases/download/${version}/${asset}"
fi

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

echo "Descargando ${asset} (${version})..."
if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$url" -o "$tmp_dir/$asset"
elif command -v wget >/dev/null 2>&1; then
  wget -q "$url" -O "$tmp_dir/$asset"
else
  echo "Necesito curl o wget instalado para continuar." >&2
  exit 1
fi

tar -xzf "$tmp_dir/$asset" -C "$tmp_dir"

install_dir="${HELS_INSTALL_DIR:-/usr/local/bin}"
if [ ! -w "$install_dir" ] 2>/dev/null; then
  install_dir="$HOME/.local/bin"
  mkdir -p "$install_dir"
fi

mv "$tmp_dir/$BIN_NAME" "$install_dir/$BIN_NAME"
chmod +x "$install_dir/$BIN_NAME"

echo "hels instalado en $install_dir/$BIN_NAME"

# Agrega install_dir al PATH de forma idempotente en un archivo de shell.
add_to_path_rc() {
  rc_file="$1"
  line="export PATH=\"$install_dir:\$PATH\""

  if [ -f "$rc_file" ] && grep -qF "$install_dir" "$rc_file" 2>/dev/null; then
    return 0
  fi

  {
    echo ""
    echo "# hels: agregado por install.sh"
    echo "$line"
  } >> "$rc_file"
  echo "  -> agregado a $rc_file"
}

case ":$PATH:" in
  *":$install_dir:"*) ;;
  *)
    echo ""
    echo "$install_dir no estaba en tu PATH. Ajustando tu shell:"
    shell_name="$(basename "${SHELL:-sh}")"
    case "$shell_name" in
      zsh)
        add_to_path_rc "$HOME/.zshrc"
        add_to_path_rc "$HOME/.zprofile"
        ;;
      bash)
        add_to_path_rc "$HOME/.bashrc"
        add_to_path_rc "$HOME/.profile"
        ;;
      *)
        add_to_path_rc "$HOME/.profile"
        ;;
    esac
    echo "  Abrí una terminal nueva (o una nueva sesión SSH) para que tome efecto."
    echo ""
    export PATH="$install_dir:$PATH"
    ;;
esac

"$install_dir/$BIN_NAME" version
