# Transcribation

Push-to-talk voice transcription in the terminal.
Hold a key — speak — release — get text. With optional translation to English.

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
- Install Python and ffmpeg if missing
- Set up an isolated environment in `~/.transcribation/`
- Install all dependencies automatically
- Set up the `transcribe` command
- Optionally download the Whisper model

## Usage

```bash
transcribe
```

Interactive menu lets you pick model, language, translation, and recording mode.
Then it enters a loop:

1. **Hold any key** while speaking
2. **Release** — transcription happens
3. **Result** appears in terminal + copied to clipboard
4. Repeat. `Ctrl+C` to exit.

### Quick start with flags

```bash
transcribe -m small -l auto          # Small model, auto-detect language
transcribe -t                        # Translate everything to English
transcribe -m large-v3 --device cuda # Large model on GPU
transcribe --mode toggle             # Press Enter to start/stop instead of hold
```

### All options

| Flag | Short | Description |
|------|-------|-------------|
| `--model MODEL` | `-m` | Model: `tiny`, `base`, `small`, `medium`, `large-v3` |
| `--language LANG` | `-l` | Source language code (`ru`, `en`, `auto`, etc.) |
| `--translate` | `-t` | Translate to English |
| `--mode MODE` | | `hold` (default) or `toggle` |
| `--device DEVICE` | | `auto`, `cuda`, or `cpu` |
| `--list-languages` | | Show all language codes |

## Models

| Model | Size | Speed | Quality | VRAM |
|-------|------|-------|---------|------|
| tiny | ~75 MB | Fastest | Low | ~1 GB |
| base | ~150 MB | Fast | OK | ~1 GB |
| small | ~500 MB | Medium | Good | ~2 GB |
| medium | ~1.5 GB | Slow | Great | ~5 GB |
| large-v3 | ~3 GB | Slowest | Best | ~10 GB |

Models download automatically on first use and cache in `~/.cache/huggingface/`.

## Translation

Whisper has a built-in translate mode that converts any language to English.
Speak in any of 99 supported languages — get English text output.

```bash
transcribe -t
```

## Supported Languages

`en` English, `ru` Russian, `es` Spanish, `fr` French, `de` German,
`zh` Chinese, `ja` Japanese, `ko` Korean, `pt` Portuguese, `it` Italian,
`uk` Ukrainian, `pl` Polish, `ar` Arabic, `hi` Hindi, `tr` Turkish

Run `transcribe --list-languages` for more.

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
