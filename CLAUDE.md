# Transcribation - Project Configuration

## Project Overview

**Transcribation** - кросс-платформенный CLI-инструмент для транскрипции и перевода речи.
Устанавливается одной командой из GitHub. Работает локально через faster-whisper.

## Architecture

- **transcribe.py** - единый Python-скрипт, точка входа
- **install.sh** - установщик для Linux/macOS (curl one-liner)
- **install.ps1** - установщик для Windows (irm one-liner)
- Всё работает через **faster-whisper** (CTranslate2, быстрее оригинального Whisper)
- Модели скачиваются автоматически при первом запуске

## Tech Stack

- **Python 3.9+** - основной язык
- **faster-whisper** - движок транскрипции (Whisper на CTranslate2)
- **sounddevice** - запись с микрофона
- **rich** - красивый вывод в терминале
- **ffmpeg** - обработка аудио/видео (внешняя зависимость)

## Design Principles

1. **Одна команда для установки** - пользователь копирует одну строку из README и всё работает
2. **Кросс-платформенность** - Linux, macOS, Windows без изменений в основном коде
3. **Локальная работа** - никаких API ключей, всё работает офлайн
4. **Минимализм** - один файл, минимум зависимостей, никакого оверинжиниринга
5. **Интерактивность** - красивые меню выбора в терминале при необходимости
6. **GPU-ускорение** - автоматически использует CUDA если доступен

## Conventions

- Весь код в одном файле `transcribe.py` - не разбивать на модули
- Установщики максимально автономные - определяют ОС и ставят всё сами
- README содержит copy-paste команды для каждой ОС
- Никаких внешних API или платных сервисов
- Whisper translate task для перевода на английский (встроенная функция модели)
- Логирование через print с цветами, не через logging module

## File Structure

```
transcribation/
├── CLAUDE.md           # Этот файл - конфиг для Claude Code
├── README.md           # Документация с командами установки
├── LICENSE             # MIT License
├── install.sh          # Установщик Linux/macOS
├── install.ps1         # Установщик Windows
├── transcribe.py       # Главный скрипт
├── requirements.txt    # Python зависимости
└── .gitignore          # Git ignore
```

## Key Features

- Транскрипция аудио/видео файлов
- Запись с микрофона в реальном времени
- Перевод речи на английский (Whisper translate)
- Интерактивный выбор языка, модели, формата вывода
- Форматы вывода: txt, srt, vtt, json
- Размеры моделей: tiny, base, small, medium, large-v3
- Авто-определение GPU (CUDA) / CPU
