#!/usr/bin/env bash
set -euo pipefail

REPO="UberMorgott/morgottalk"
INSTALL_DIR="/usr/local/bin"
BIN_NAME="morgottalk"

# --- Colors ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { echo -e "${CYAN}[*]${NC} $*"; }
ok()    { echo -e "${GREEN}[+]${NC} $*"; }
warn()  { echo -e "${YELLOW}[!]${NC} $*"; }
error() { echo -e "${RED}[-]${NC} $*"; exit 1; }

# --- Detect OS & arch ---
OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
    Linux)  PLATFORM="linux" ;;
    Darwin) PLATFORM="macos" ;;
    MINGW*|MSYS*|CYGWIN*) PLATFORM="windows" ;;
    *) error "Unsupported OS: $OS" ;;
esac

case "$ARCH" in
    x86_64|amd64) ARCH_TAG="amd64" ;;
    aarch64|arm64) ARCH_TAG="arm64" ;;
    *) error "Unsupported architecture: $ARCH" ;;
esac

# --- Pick asset name ---
case "$PLATFORM" in
    linux)   ASSET="${BIN_NAME}-linux-${ARCH_TAG}" ;;
    macos)   ASSET="${BIN_NAME}-macos-${ARCH_TAG}" ;;
    windows) ASSET="${BIN_NAME}-windows-${ARCH_TAG}.exe" ;;
esac

DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"

info "MorgoTTalk installer"
info "OS: ${PLATFORM}, Arch: ${ARCH_TAG}"
echo ""

# --- Install system dependencies (Linux only) ---
if [ "$PLATFORM" = "linux" ]; then
    install_deps() {
        if command -v pacman &>/dev/null; then
            info "Installing dependencies via pacman..."
            sudo pacman -S --needed --noconfirm webkit2gtk-4.1 gtk3
        elif command -v apt-get &>/dev/null; then
            info "Installing dependencies via apt..."
            sudo apt-get update -qq
            sudo apt-get install -y -qq libwebkit2gtk-4.1-dev libgtk-3-dev
        elif command -v dnf &>/dev/null; then
            info "Installing dependencies via dnf..."
            sudo dnf install -y webkit2gtk4.1-devel gtk3-devel
        elif command -v zypper &>/dev/null; then
            info "Installing dependencies via zypper..."
            sudo zypper install -y webkit2gtk-soup2-4_1-devel gtk3-devel
        else
            warn "Could not detect package manager. Please install webkit2gtk-4.1 and gtk3 manually."
        fi
    }

    # Check if webkit2gtk is already available
    if ! ldconfig -p 2>/dev/null | grep -q libwebkit2gtk-4.1 && \
       ! pkg-config --exists webkit2gtk-4.1 2>/dev/null; then
        info "webkit2gtk-4.1 not found, installing dependencies..."
        install_deps
    else
        ok "Dependencies already installed"
    fi
fi

# --- Download binary ---
info "Downloading ${ASSET}..."
TMPFILE="$(mktemp)"
trap 'rm -f "$TMPFILE"' EXIT

if command -v curl &>/dev/null; then
    curl -fSL --progress-bar -o "$TMPFILE" "$DOWNLOAD_URL"
elif command -v wget &>/dev/null; then
    wget -q --show-progress -O "$TMPFILE" "$DOWNLOAD_URL"
else
    error "Neither curl nor wget found. Please install one of them."
fi

# --- Install binary ---
if [ "$PLATFORM" = "windows" ]; then
    DEST="${INSTALL_DIR}/${BIN_NAME}.exe"
else
    DEST="${INSTALL_DIR}/${BIN_NAME}"
fi

info "Installing to ${DEST} (may ask for password)..."
sudo install -m 755 "$TMPFILE" "$DEST"

ok "MorgoTTalk installed to ${DEST}"

# --- Create .desktop entry (Linux) ---
if [ "$PLATFORM" = "linux" ]; then
    DESKTOP_DIR="${HOME}/.local/share/applications"
    mkdir -p "$DESKTOP_DIR"
    cat > "${DESKTOP_DIR}/morgottalk.desktop" <<DESKTOP
[Desktop Entry]
Name=MorgoTTalk
Comment=Push-to-talk voice transcription
Exec=${DEST}
Type=Application
Categories=Utility;Audio;
StartupNotify=true
DESKTOP
    ok "Desktop entry created"
fi

echo ""
ok "Done! Run with: ${BIN_NAME}"
echo ""
info "On first launch, open Settings and download a whisper model."
info "Then create a preset, set a hotkey, and start speaking."
