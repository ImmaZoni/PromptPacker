#!/usr/bin/env bash
set -euo pipefail

REPO="immazoni/promptpacker"
BASE_URL="https://github.com/${REPO}/releases/latest/download"

OS="$(uname -s)"
ARCH="$(uname -m)"

case "${OS}" in
  Linux)   os="linux" ;;
  Darwin)  os="darwin" ;;
  *)
    echo "Unsupported OS: ${OS}. This installer currently supports Linux and macOS only." >&2
    exit 1
    ;;
esac

case "${ARCH}" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *)
    echo "Unsupported architecture: ${ARCH}. This installer currently supports amd64 and arm64 only." >&2
    exit 1
    ;;
esac

asset="promptpacker-${os}-${arch}"
echo "Detected platform: ${os}/${arch}"
echo "Selected release asset: ${asset}"

tmp_bin="$(mktemp)"
trap 'rm -f "${tmp_bin}"' EXIT

echo "Downloading latest promptpacker release..."
curl -fsSL "${BASE_URL}/${asset}" -o "${tmp_bin}"

chmod +x "${tmp_bin}"

INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
echo "Installing to ${INSTALL_DIR}..."
mkdir -p "${INSTALL_DIR}"

if [ -w "${INSTALL_DIR}" ]; then
  mv "${tmp_bin}" "${INSTALL_DIR}/promptpacker"
else
  echo "Elevated permissions required to write to ${INSTALL_DIR}."
  sudo mv "${tmp_bin}" "${INSTALL_DIR}/promptpacker"
fi

echo "Installed promptpacker to ${INSTALL_DIR}/promptpacker"

if command -v promptpacker >/dev/null 2>&1; then
  echo "You can now run 'promptpacker' from any terminal."
else
  echo
  echo "NOTE: '${INSTALL_DIR}' does not appear to be on your PATH."
  echo "Add it to your PATH or move 'promptpacker' to a directory that is already on PATH."
fi

