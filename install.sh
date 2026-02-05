#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════
#  Transcribation Installer — Linux / macOS
#  Usage: curl -sSL https://raw.githubusercontent.com/UberMorgott/transcribation/main/install.sh | bash
# ═══════════════════════════════════════════════════════════════════════════

set -euo pipefail

# ── Colors ──────────────────────────────────────────────────────────────────

RED='\033[91m'
GREEN='\033[92m'
YELLOW='\033[93m'
BLUE='\033[94m'
CYAN='\033[96m'
BOLD='\033[1m'
DIM='\033[2m'
RESET='\033[0m'

info()    { echo -e "  ${BLUE}ℹ${RESET}  $1"; }
success() { echo -e "  ${GREEN}✓${RESET}  $1"; }
warn()    { echo -e "  ${YELLOW}⚠${RESET}  $1"; }
error()   { echo -e "  ${RED}✗${RESET}  $1"; }
die()     { error "$1"; exit 1; }

# ── Banner ──────────────────────────────────────────────────────────────────

echo -e "
${CYAN}${BOLD}╔══════════════════════════════════════════╗
║       🎤  Transcribation Installer       ║
║    Speech transcription & translation    ║
╚══════════════════════════════════════════╝${RESET}
"

# ── Detect OS ───────────────────────────────────────────────────────────────

OS="unknown"
PKG=""

if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    OS="linux"
    if command -v pacman &>/dev/null; then
        PKG="pacman"
    elif command -v apt-get &>/dev/null; then
        PKG="apt"
    elif command -v dnf &>/dev/null; then
        PKG="dnf"
    elif command -v zypper &>/dev/null; then
        PKG="zypper"
    fi
elif [[ "$OSTYPE" == "darwin"* ]]; then
    OS="macos"
    if command -v brew &>/dev/null; then
        PKG="brew"
    fi
fi

info "OS: ${BOLD}${OS}${RESET} | Package manager: ${BOLD}${PKG:-none}${RESET}"

# ── Install directory ───────────────────────────────────────────────────────

INSTALL_DIR="$HOME/.transcribation"
BIN_DIR="$HOME/.local/bin"

info "Install directory: ${BOLD}${INSTALL_DIR}${RESET}"

# ── Check / install Python ──────────────────────────────────────────────────

install_python() {
    if command -v python3 &>/dev/null; then
        PY=$(command -v python3)
        PY_VER=$($PY --version 2>&1 | awk '{print $2}')
        success "Python found: ${BOLD}${PY_VER}${RESET}"
        return
    fi

    warn "Python 3 not found, installing..."

    case "$PKG" in
        pacman) sudo pacman -S --noconfirm python ;;
        apt)    sudo apt-get update && sudo apt-get install -y python3 python3-venv python3-pip ;;
        dnf)    sudo dnf install -y python3 python3-pip ;;
        zypper) sudo zypper install -y python3 python3-pip ;;
        brew)   brew install python ;;
        *)      die "Cannot install Python automatically. Install Python 3.9+ manually." ;;
    esac

    command -v python3 &>/dev/null || die "Python installation failed"
    success "Python installed"
}

# ── Check / install ffmpeg ──────────────────────────────────────────────────

install_ffmpeg() {
    if command -v ffmpeg &>/dev/null; then
        FF_VER=$(ffmpeg -version 2>&1 | head -1 | awk '{print $3}')
        success "ffmpeg found: ${BOLD}${FF_VER}${RESET}"
        return
    fi

    warn "ffmpeg not found, installing..."

    case "$PKG" in
        pacman) sudo pacman -S --noconfirm ffmpeg ;;
        apt)    sudo apt-get update && sudo apt-get install -y ffmpeg ;;
        dnf)    sudo dnf install -y ffmpeg ;;
        zypper) sudo zypper install -y ffmpeg ;;
        brew)   brew install ffmpeg ;;
        *)      die "Cannot install ffmpeg automatically. Install it manually." ;;
    esac

    command -v ffmpeg &>/dev/null || die "ffmpeg installation failed"
    success "ffmpeg installed"
}

# ── Check / install portaudio (for sounddevice) ────────────────────────────

install_portaudio() {
    # Check if portaudio is already available
    local need_install=false

    case "$OS" in
        linux)
            if ! ldconfig -p 2>/dev/null | grep -q libportaudio; then
                need_install=true
            fi
            ;;
        macos)
            if ! brew list portaudio &>/dev/null 2>&1; then
                need_install=true
            fi
            ;;
    esac

    if [ "$need_install" = true ]; then
        info "Installing PortAudio (required for microphone recording)..."
        case "$PKG" in
            pacman) sudo pacman -S --noconfirm portaudio ;;
            apt)    sudo apt-get install -y portaudio19-dev ;;
            dnf)    sudo dnf install -y portaudio-devel ;;
            zypper) sudo zypper install -y portaudio-devel ;;
            brew)   brew install portaudio ;;
            *)      warn "Cannot install PortAudio automatically. Microphone may not work." ;;
        esac
        success "PortAudio installed"
    else
        success "PortAudio found"
    fi
}

# ── Model selection ─────────────────────────────────────────────────────────

select_model() {
    echo -e "\n  ${BOLD}Select Whisper model:${RESET}"
    echo -e "  ${DIM}[1]${RESET} ${BOLD}tiny${RESET}     ${DIM}— ~75 MB,  fastest,  lowest quality${RESET}"
    echo -e "  ${DIM}[2]${RESET} ${BOLD}base${RESET}     ${DIM}— ~150 MB, fast,     decent quality${RESET}"
    echo -e "  ${DIM}[3]${RESET} ${BOLD}small${RESET}    ${DIM}— ~500 MB, balanced, good quality${RESET}"
    echo -e "  ${DIM}[4]${RESET} ${BOLD}medium${RESET}   ${DIM}— ~1.5 GB, slower,   great quality${RESET}"
    echo -e "  ${DIM}[5]${RESET} ${BOLD}large-v3${RESET} ${DIM}— ~3 GB,   slowest,  best quality${RESET}"
    echo ""

    while true; do
        read -rp "  $(echo -e ${CYAN})>${RESET} Choose [1-5] (default 3): " choice
        choice=${choice:-3}
        case "$choice" in
            1) MODEL="tiny";     break ;;
            2) MODEL="base";     break ;;
            3) MODEL="small";    break ;;
            4) MODEL="medium";   break ;;
            5) MODEL="large-v3"; break ;;
            *) echo -e "  ${RED}Invalid choice${RESET}" ;;
        esac
    done

    success "Model: ${BOLD}${MODEL}${RESET}"
}

# ── GPU detection ───────────────────────────────────────────────────────────

detect_gpu() {
    GPU_AVAILABLE=false

    if command -v nvidia-smi &>/dev/null; then
        GPU_NAME=$(nvidia-smi --query-gpu=name --format=csv,noheader,nounits 2>/dev/null | head -1)
        if [ -n "$GPU_NAME" ]; then
            GPU_AVAILABLE=true
            success "NVIDIA GPU detected: ${BOLD}${GPU_NAME}${RESET}"
        fi
    fi

    if [ "$GPU_AVAILABLE" = false ]; then
        info "No NVIDIA GPU detected — will use CPU"
    fi
}

# ── Setup virtual environment ───────────────────────────────────────────────

setup_venv() {
    info "Setting up Python virtual environment..."

    # Clean previous install
    if [ -d "$INSTALL_DIR" ]; then
        warn "Previous installation found, removing..."
        rm -rf "$INSTALL_DIR"
    fi

    mkdir -p "$INSTALL_DIR"
    python3 -m venv "$INSTALL_DIR/venv"
    source "$INSTALL_DIR/venv/bin/activate"

    # Upgrade pip
    pip install --upgrade pip --quiet

    success "Virtual environment created"
}

# ── Install Python packages ─────────────────────────────────────────────────

install_packages() {
    info "Installing Python packages..."

    # Base packages
    pip install --quiet faster-whisper sounddevice numpy

    # CUDA support if GPU available
    if [ "$GPU_AVAILABLE" = true ]; then
        info "Installing CUDA support for GPU acceleration..."
        pip install --quiet nvidia-cublas-cu12 nvidia-cudnn-cu12 2>/dev/null || \
            warn "CUDA Python packages not installed (may already be system-wide)"
    fi

    success "Python packages installed"
}

# ── Install transcribe script ───────────────────────────────────────────────

install_script() {
    info "Installing transcribe command..."

    # Download the script
    REPO_URL="https://raw.githubusercontent.com/UberMorgott/transcribation/main"
    curl -sSL "$REPO_URL/transcribe.py" -o "$INSTALL_DIR/transcribe.py"
    chmod +x "$INSTALL_DIR/transcribe.py"

    # Create wrapper script
    mkdir -p "$BIN_DIR"

    cat > "$BIN_DIR/transcribe" << 'WRAPPER'
#!/usr/bin/env bash
source "$HOME/.transcribation/venv/bin/activate"
python "$HOME/.transcribation/transcribe.py" "$@"
WRAPPER

    chmod +x "$BIN_DIR/transcribe"

    # Add to PATH if needed
    if [[ ":$PATH:" != *":$BIN_DIR:"* ]]; then
        warn "$BIN_DIR is not in PATH"

        SHELL_RC=""
        if [ -f "$HOME/.zshrc" ]; then
            SHELL_RC="$HOME/.zshrc"
        elif [ -f "$HOME/.bashrc" ]; then
            SHELL_RC="$HOME/.bashrc"
        fi

        if [ -n "$SHELL_RC" ]; then
            if ! grep -q '.local/bin' "$SHELL_RC"; then
                echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$SHELL_RC"
                info "Added $BIN_DIR to PATH in $SHELL_RC"
            fi
        fi

        export PATH="$BIN_DIR:$PATH"
    fi

    success "Command installed: ${BOLD}transcribe${RESET}"
}

# ── Pre-download model ──────────────────────────────────────────────────────

predownload_model() {
    echo ""
    read -rp "  $(echo -e ${CYAN})>${RESET} Download model now? [Y/n] " dl
    dl=${dl:-y}

    if [[ "$dl" =~ ^[Yy] ]]; then
        info "Downloading model ${BOLD}${MODEL}${RESET}... (this may take a while)"
        source "$INSTALL_DIR/venv/bin/activate"
        python3 -c "
from faster_whisper import WhisperModel
print('  Downloading...')
model = WhisperModel('${MODEL}', device='cpu', compute_type='int8')
print('  Done!')
" 2>&1 | while IFS= read -r line; do echo "  $line"; done
        success "Model downloaded"
    else
        info "Model will be downloaded on first run"
    fi
}

# ── Main ────────────────────────────────────────────────────────────────────

main() {
    install_python
    install_ffmpeg
    install_portaudio
    detect_gpu
    select_model
    setup_venv
    install_packages
    install_script
    predownload_model

    echo -e "
${GREEN}${BOLD}╔══════════════════════════════════════════╗
║         ✓  Installation complete!        ║
╚══════════════════════════════════════════╝${RESET}

  ${BOLD}Usage:${RESET}
    ${CYAN}transcribe${RESET}                    Interactive mode
    ${CYAN}transcribe file.mp3${RESET}           Transcribe a file
    ${CYAN}transcribe --mic${RESET}              Record & transcribe
    ${CYAN}transcribe -t video.mp4${RESET}       Translate to English
    ${CYAN}transcribe --help${RESET}             Show all options

  ${DIM}If 'transcribe' is not found, restart your terminal
  or run: export PATH=\"\$HOME/.local/bin:\$PATH\"${RESET}
"
}

main
