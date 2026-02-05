# ═══════════════════════════════════════════════════════════════════════════
#  Transcribation Installer — Windows (PowerShell)
#  Usage: irm https://raw.githubusercontent.com/UberMorgott/transcribation/main/install.ps1 | iex
# ═══════════════════════════════════════════════════════════════════════════

$ErrorActionPreference = "Stop"

# ── Helpers ─────────────────────────────────────────────────────────────────

function Write-Info    { Write-Host "  i  " -ForegroundColor Blue -NoNewline; Write-Host $args[0] }
function Write-Ok      { Write-Host "  +  " -ForegroundColor Green -NoNewline; Write-Host $args[0] }
function Write-Warn    { Write-Host "  !  " -ForegroundColor Yellow -NoNewline; Write-Host $args[0] }
function Write-Err     { Write-Host "  x  " -ForegroundColor Red -NoNewline; Write-Host $args[0] }

function Write-Banner {
    Write-Host ""
    Write-Host "  ══════════════════════════════════════════" -ForegroundColor Cyan
    Write-Host "         Transcribation Installer           " -ForegroundColor Cyan
    Write-Host "    Speech transcription & translation      " -ForegroundColor Cyan
    Write-Host "  ══════════════════════════════════════════" -ForegroundColor Cyan
    Write-Host ""
}

# ── Variables ───────────────────────────────────────────────────────────────

$InstallDir = "$env:USERPROFILE\.transcribation"
$BinDir     = "$env:USERPROFILE\.local\bin"
$RepoUrl    = "https://raw.githubusercontent.com/UberMorgott/transcribation/main"

# ── Check / install Python ──────────────────────────────────────────────────

function Install-Python {
    $py = Get-Command python -ErrorAction SilentlyContinue
    if (-not $py) { $py = Get-Command python3 -ErrorAction SilentlyContinue }

    if ($py) {
        $ver = & $py.Source --version 2>&1
        Write-Ok "Python found: $ver"
        return $py.Source
    }

    Write-Warn "Python not found, attempting to install..."

    # Try winget first
    $winget = Get-Command winget -ErrorAction SilentlyContinue
    if ($winget) {
        Write-Info "Installing Python via winget..."
        winget install Python.Python.3.12 --accept-source-agreements --accept-package-agreements
    }
    # Try chocolatey
    elseif (Get-Command choco -ErrorAction SilentlyContinue) {
        Write-Info "Installing Python via Chocolatey..."
        choco install python -y
    }
    else {
        Write-Err "Cannot install Python automatically."
        Write-Err "Please install Python 3.9+ from https://www.python.org/downloads/"
        Write-Err "Make sure to check 'Add to PATH' during installation!"
        exit 1
    }

    # Refresh PATH
    $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")

    $py = Get-Command python -ErrorAction SilentlyContinue
    if (-not $py) {
        Write-Err "Python installation failed. Please install manually."
        exit 1
    }

    Write-Ok "Python installed"
    return $py.Source
}

# ── Check / install ffmpeg ──────────────────────────────────────────────────

function Install-FFmpeg {
    if (Get-Command ffmpeg -ErrorAction SilentlyContinue) {
        $ver = (ffmpeg -version 2>&1 | Select-Object -First 1) -replace "ffmpeg version ", ""
        Write-Ok "ffmpeg found: $ver"
        return
    }

    Write-Warn "ffmpeg not found, attempting to install..."

    if (Get-Command winget -ErrorAction SilentlyContinue) {
        winget install Gyan.FFmpeg --accept-source-agreements --accept-package-agreements
    }
    elseif (Get-Command choco -ErrorAction SilentlyContinue) {
        choco install ffmpeg -y
    }
    else {
        Write-Err "Cannot install ffmpeg automatically."
        Write-Err "Download from https://ffmpeg.org/download.html and add to PATH"
        exit 1
    }

    $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")
    Write-Ok "ffmpeg installed"
}

# ── GPU detection ───────────────────────────────────────────────────────────

function Test-GPU {
    $script:GpuAvailable = $false
    try {
        $nvsmi = Get-Command nvidia-smi -ErrorAction SilentlyContinue
        if ($nvsmi) {
            $gpuName = (nvidia-smi --query-gpu=name --format=csv,noheader,nounits 2>$null | Select-Object -First 1).Trim()
            if ($gpuName) {
                $script:GpuAvailable = $true
                Write-Ok "NVIDIA GPU detected: $gpuName"
                return
            }
        }
    } catch {}
    Write-Info "No NVIDIA GPU detected — will use CPU"
}

# ── Model selection ─────────────────────────────────────────────────────────

function Select-Model {
    Write-Host ""
    Write-Host "  Select Whisper model:" -ForegroundColor White
    Write-Host "  [1] tiny     — ~75 MB,  fastest,  lowest quality" -ForegroundColor DarkGray
    Write-Host "  [2] base     — ~150 MB, fast,     decent quality" -ForegroundColor DarkGray
    Write-Host "  [3] small    — ~500 MB, balanced, good quality" -ForegroundColor DarkGray
    Write-Host "  [4] medium   — ~1.5 GB, slower,   great quality" -ForegroundColor DarkGray
    Write-Host "  [5] large-v3 — ~3 GB,   slowest,  best quality" -ForegroundColor DarkGray
    Write-Host ""

    do {
        $choice = Read-Host "  > Choose [1-5] (default 3)"
        if (-not $choice) { $choice = "3" }
    } while ($choice -notmatch '^[1-5]$')

    $models = @{ "1"="tiny"; "2"="base"; "3"="small"; "4"="medium"; "5"="large-v3" }
    $script:Model = $models[$choice]
    Write-Ok "Model: $script:Model"
}

# ── Setup virtual environment ───────────────────────────────────────────────

function Setup-VEnv {
    param([string]$PythonExe)

    Write-Info "Setting up Python virtual environment..."

    if (Test-Path $InstallDir) {
        Write-Warn "Previous installation found, removing..."
        Remove-Item -Recurse -Force $InstallDir
    }

    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null

    & $PythonExe -m venv "$InstallDir\venv"
    & "$InstallDir\venv\Scripts\pip.exe" install --upgrade pip --quiet 2>$null

    Write-Ok "Virtual environment created"
}

# ── Install Python packages ─────────────────────────────────────────────────

function Install-Packages {
    Write-Info "Installing Python packages..."

    & "$InstallDir\venv\Scripts\pip.exe" install --quiet faster-whisper sounddevice numpy

    if ($script:GpuAvailable) {
        Write-Info "Installing CUDA support for GPU acceleration..."
        try {
            & "$InstallDir\venv\Scripts\pip.exe" install --quiet nvidia-cublas-cu12 nvidia-cudnn-cu12 2>$null
        } catch {
            Write-Warn "CUDA Python packages not installed (may already be system-wide)"
        }
    }

    Write-Ok "Python packages installed"
}

# ── Install transcribe script ───────────────────────────────────────────────

function Install-Script {
    Write-Info "Installing transcribe command..."

    # Download the main script
    Invoke-WebRequest -Uri "$RepoUrl/transcribe.py" -OutFile "$InstallDir\transcribe.py"

    # Create bin directory
    New-Item -ItemType Directory -Path $BinDir -Force | Out-Null

    # Create batch wrapper
    @"
@echo off
"$env:USERPROFILE\.transcribation\venv\Scripts\python.exe" "$env:USERPROFILE\.transcribation\transcribe.py" %*
"@ | Set-Content "$BinDir\transcribe.cmd" -Encoding ASCII

    # Create PowerShell wrapper
    @"
& "`$env:USERPROFILE\.transcribation\venv\Scripts\python.exe" "`$env:USERPROFILE\.transcribation\transcribe.py" @args
"@ | Set-Content "$BinDir\transcribe.ps1" -Encoding UTF8

    # Add to PATH if needed
    $userPath = [System.Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -notlike "*$BinDir*") {
        [System.Environment]::SetEnvironmentVariable("Path", "$BinDir;$userPath", "User")
        $env:Path = "$BinDir;$env:Path"
        Write-Info "Added $BinDir to user PATH"
    }

    Write-Ok "Command installed: transcribe"
}

# ── Pre-download model ──────────────────────────────────────────────────────

function Download-Model {
    Write-Host ""
    $dl = Read-Host "  > Download model now? [Y/n]"
    if (-not $dl) { $dl = "Y" }

    if ($dl -match '^[Yy]') {
        Write-Info "Downloading model $script:Model... (this may take a while)"

        $pyScript = @"
from faster_whisper import WhisperModel
print('  Downloading...')
model = WhisperModel('$script:Model', device='cpu', compute_type='int8')
print('  Done!')
"@

        & "$InstallDir\venv\Scripts\python.exe" -c $pyScript
        Write-Ok "Model downloaded"
    }
    else {
        Write-Info "Model will be downloaded on first run"
    }
}

# ── Main ────────────────────────────────────────────────────────────────────

Write-Banner

$PythonExe = Install-Python
Install-FFmpeg
Test-GPU
Select-Model
Setup-VEnv -PythonExe $PythonExe
Install-Packages
Install-Script
Download-Model

Write-Host ""
Write-Host "  ══════════════════════════════════════════" -ForegroundColor Green
Write-Host "         Installation complete!             " -ForegroundColor Green
Write-Host "  ══════════════════════════════════════════" -ForegroundColor Green
Write-Host ""
Write-Host "  Usage:" -ForegroundColor White
Write-Host "    transcribe                    Interactive mode" -ForegroundColor Cyan
Write-Host "    transcribe file.mp3           Transcribe a file" -ForegroundColor Cyan
Write-Host "    transcribe --mic              Record & transcribe" -ForegroundColor Cyan
Write-Host "    transcribe -t video.mp4       Translate to English" -ForegroundColor Cyan
Write-Host "    transcribe --help             Show all options" -ForegroundColor Cyan
Write-Host ""
Write-Host "  If 'transcribe' is not found, restart your terminal." -ForegroundColor DarkGray
Write-Host ""
