# Transcribation - Project Configuration

## Project Overview

**Transcribation** — кросс-платформенное десктопное приложение push-to-talk для транскрипции и перевода голоса.
Один портативный бинарник. Системный трей. Глобальный хоткей.

## Architecture

- **Go + CGO** — бэкенд, единый бинарник
- **Wails v3** — десктопный фреймворк (трей, окно, биндинги)
- **Svelte + TypeScript + Tailwind** — фронтенд UI
- **whisper.cpp** — STT движок (git submodule, статическая линковка)
- **malgo (miniaudio)** — захват звука, 0 зависимостей
- **golang.design/x/hotkey** — глобальные горячие клавиши
- **golang.design/x/clipboard** — кросс-платформенный буфер обмена

## Tech Stack

| Компонент | Технология |
|-----------|-----------|
| Бэкенд | Go 1.25+ CGO |
| GUI | Wails v3 alpha |
| Фронтенд | Svelte + TypeScript + Tailwind |
| STT | whisper.cpp (Go bindings) |
| Аудио | malgo (miniaudio) |
| Хоткей | golang.design/x/hotkey |
| CI/CD | GitHub Actions |

## Design Principles

1. **Один бинарник** — скачал, запустил, работает
2. **Кросс-платформенность** — Linux, macOS, Windows
3. **Локальная работа** — никаких API, всё офлайн
4. **Минимум зависимостей** — только webkit2gtk на Linux
5. **Портативность** — модели скачиваются при первом запуске

## File Structure

```
transcribation/
├── main.go                     # Wails app entry point
├── services/
│   ├── transcription.go        # Recording → transcription → clipboard
│   ├── settings.go             # Settings service for frontend
│   ├── audio.go                # malgo audio capture
│   ├── whisper.go              # whisper.cpp wrapper
│   └── models.go               # Model downloading
├── internal/
│   ├── hotkey/                 # Global hotkey (platform-specific)
│   ├── clipboard/              # Clipboard operations
│   └── config/                 # JSON config persistence
├── third_party/
│   └── whisper.cpp/            # Git submodule
├── frontend/
│   ├── src/                    # Svelte + TypeScript
│   └── package.json
├── build/                      # Wails build configs
├── scripts/
│   └── build-whisper.sh        # Build libwhisper.a
├── CLAUDE.md
├── README.md
├── go.mod
└── Taskfile.yml
```

## Conventions

- Go код — стандартный стиль (gofmt)
- Фронтенд — Svelte компоненты, TypeScript strict
- Конфиг — JSON в стандартной директории ОС
- Модели — GGML формат, скачиваются из Hugging Face
- Не использовать устаревшие библиотеки
- Платформо-зависимый код через build tags
