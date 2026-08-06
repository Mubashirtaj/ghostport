#!/usr/bin/env bash
#
# Installs the latest release of ghostport into /usr/local/bin.
#
#   curl -fsSL https://raw.githubusercontent.com/mubashirtaj/ghostport/main/install.sh | bash
#
set -euo pipefail

REPO="mubashirtaj/ghostport"
BIN_NAME="ghostport"
INSTALL_DIR="/usr/local/bin"

log() { printf '\033[1;35m==>\033[0m %s\n' "$1"; }
err() { printf '\033[1;31mERROR:\033[0m %s\n' "$1" >&2; exit 1; }

command -v curl >/dev/null 2>&1 || err "curl is required but not installed."
command -v tar >/dev/null 2>&1 || err "tar is required but not installed."

# --- Detect OS ---------------------------------------------------------
OS_RAW="$(uname -s)"
case "$OS_RAW" in
  Linux*)  OS="linux" ;;
  Darwin*) OS="darwin" ;;
  *) err "Unsupported operating system: $OS_RAW" ;;
esac

# --- Detect architecture -------------------------------------------------
ARCH_RAW="$(uname -m)"
case "$ARCH_RAW" in
  x86_64|amd64)   ARCH="amd64" ;;
  aarch64|arm64)  ARCH="arm64" ;;
  *) err "Unsupported architecture: $ARCH_RAW" ;;
esac

log "Detected platform: ${OS}/${ARCH}"

# --- Resolve latest version ----------------------------------------------
log "Fetching latest release information..."
VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name":' \
  | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"

[ -n "$VERSION" ] || err "Could not determine the latest ghostport version."
log "Latest version: ${VERSION}"

# --- Download and extract -------------------------------------------------
VERSION_NUM="${VERSION#v}"
ARCHIVE="${BIN_NAME}_${VERSION_NUM}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

TMP_DIR="$(mktemp -d "/tmp/${BIN_NAME}.XXXXXX")"
trap 'rm -rf "$TMP_DIR"' EXIT

log "Downloading ${URL}"
curl -fsSL "$URL" -o "${TMP_DIR}/${ARCHIVE}" || err "Failed to download ${URL}"

log "Extracting archive..."
tar -xzf "${TMP_DIR}/${ARCHIVE}" -C "$TMP_DIR"

[ -f "${TMP_DIR}/${BIN_NAME}" ] || err "Binary not found in downloaded archive."
chmod +x "${TMP_DIR}/${BIN_NAME}"

# --- Install ---------------------------------------------------------------
log "Installing to ${INSTALL_DIR}/${BIN_NAME} (sudo may prompt for your password)"
sudo mv "${TMP_DIR}/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"

log "ghostport ${VERSION} installed successfully!"
"${INSTALL_DIR}/${BIN_NAME}" --version
