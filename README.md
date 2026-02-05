# Transcribation

Cross-platform CLI tool for speech transcription and translation.
Works locally, offline, with GPU acceleration. Powered by [faster-whisper](https://github.com/SYSTRAN/faster-whisper).

## Quick Install

### Linux / macOS

```bash
curl -sSL https://raw.githubusercontent.com/UberMorgott/transcribation/main/install.sh | bash
```

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/UberMorgott/transcribation/main/install.ps1 | iex
```

The installer will:
- Install Python 3 and ffmpeg if missing
- Create a virtual environment in `~/.transcribation/`
- Install all dependencies
- Set up the `transcribe` command
- Optionally download the Whisper model

## Usage

### Interactive mode

```bash
transcribe
```

Launches a menu where you pick the mode, model, language, and output format.

### Transcribe a file

```bash
transcribe audio.mp3
transcribe video.mp4
transcribe lecture.wav
```

### Record from microphone

```bash
transcribe --mic                  # Press Enter to stop
transcribe --mic --duration 30    # Record 30 seconds
```

### Translate to English

```bash
transcribe --translate video.mp4
transcribe -t --mic
```

Whisper translates any language to English using its built-in translate task.

### Choose model size

```bash
transcribe -m tiny audio.mp3     # Fastest, lowest quality
transcribe -m base audio.mp3
transcribe -m small audio.mp3    # Default, balanced
transcribe -m medium audio.mp3
transcribe -m large-v3 audio.mp3 # Best quality, needs more RAM/VRAM
```

### Specify source language

```bash
transcribe -l ru lecture.mp3     # Russian
transcribe -l ja anime.mp4      # Japanese
transcribe --list-languages      # Show all language codes
```

### Output formats

```bash
transcribe -f txt audio.mp3     # Plain text (default)
transcribe -f srt audio.mp3     # SubRip subtitles
transcribe -f vtt audio.mp3     # WebVTT subtitles
transcribe -f json audio.mp3    # JSON with timestamps
```

### Custom output path

```bash
transcribe -o output.srt -f srt audio.mp3
```

### Force CPU/GPU

```bash
transcribe --device cuda audio.mp3   # Force GPU
transcribe --device cpu audio.mp3    # Force CPU
```

## Models

| Model    | Size    | Speed   | Quality | VRAM  |
|----------|---------|---------|---------|-------|
| tiny     | ~75 MB  | Fastest | Low     | ~1 GB |
| base     | ~150 MB | Fast    | OK      | ~1 GB |
| small    | ~500 MB | Medium  | Good    | ~2 GB |
| medium   | ~1.5 GB | Slow    | Great   | ~5 GB |
| large-v3 | ~3 GB   | Slowest | Best    | ~10 GB|

Models are downloaded automatically on first use and cached in `~/.cache/huggingface/`.

## Supported Languages

Whisper supports 99 languages. Most common:

`en` English, `ru` Russian, `es` Spanish, `fr` French, `de` German,
`zh` Chinese, `ja` Japanese, `ko` Korean, `pt` Portuguese, `it` Italian,
`nl` Dutch, `pl` Polish, `uk` Ukrainian, `ar` Arabic, `hi` Hindi,
`tr` Turkish, `sv` Swedish, `cs` Czech, `vi` Vietnamese, `th` Thai

Run `transcribe --list-languages` for the full list.

## Requirements

- **Python** 3.9+
- **ffmpeg** (for audio/video processing)
- **PortAudio** (for microphone recording, Linux only)
- **NVIDIA GPU + CUDA** (optional, for faster processing)

All dependencies are installed automatically by the installer.

## Uninstall

```bash
rm -rf ~/.transcribation ~/.local/bin/transcribe
```

Windows:
```powershell
Remove-Item -Recurse "$env:USERPROFILE\.transcribation", "$env:USERPROFILE\.local\bin\transcribe.cmd", "$env:USERPROFILE\.local\bin\transcribe.ps1"
```

## License

MIT
