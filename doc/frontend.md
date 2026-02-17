# Frontend Reference

## Tech Stack

- **Svelte 4** — UI framework
- **Tailwind CSS 4** — utility-first CSS
- **Vite 5** — build tool
- **TypeScript** — type safety

## Routing

Single SPA routed via `?window=` query parameter in `App.svelte`:

| Parameter | Page | Description |
|-----------|------|-------------|
| (default) | `MainPage.svelte` | Preset cards, recording controls |
| `?window=history` | `HistoryPage.svelte` | Transcription log (separate window) |
| `?window=overlay` | `OverlayPage.svelte` | Recording indicator overlay |

## Pages

### MainPage (`pages/MainPage.svelte`)

Main application view:
- Grid of PresetCards (drag-to-reorder via SortableJS)
- "+" button to create new preset
- Gear icon → SettingsModal
- Clock icon → opens HistoryPage in separate window
- Listens to recording state events from Go backend

### HistoryPage (`pages/HistoryPage.svelte`)

Transcription history viewer:
- Opens in a separate Wails window
- Shows timestamped transcription entries
- Copy-to-clipboard per entry
- Clear all button

### OverlayPage (`pages/OverlayPage.svelte`)

Recording/processing indicator overlay:
- Vintage vacuum tube design with steampunk aesthetic
- Shows recording state (glowing tube) and processing state (spinning gears)
- Frameless, transparent, always-on-top window

## Components

### PresetCard (`components/PresetCard.svelte`)

Individual preset card with states:
- **idle** — shows preset name, model, hotkey
- **recording** — pulsing border, recording indicator
- **processing** — spinner, "transcribing..." text
- **expanded** — inline editor with all preset settings

### PresetEditor (`components/PresetEditor.svelte`)

Modal editor for preset settings:
- Name, model selection, hotkey capture
- Language (with auto-detect option)
- Input mode: hold (record while held) / toggle (press to start/stop)
- Keep model loaded toggle
- Auto-stop timer (max recording duration)

### SettingsModal (`components/SettingsModal.svelte`)

Global settings dialog:
- **Backend** — GPU backend selection pills (auto, CPU, Vulkan, CUDA, etc.)
  - Active backends: solid pill with accent color
  - Installable backends: dashed border, click to install
  - Installing: progress ring animation
  - Unavailable: dimmed, disabled
- **Theme** — dark/light toggle
- **UI Language** — 9 languages
- **Close Action** — minimize to tray / quit
- **Auto Start** — launch on system boot
- **Start Minimized** — start in tray
- **Microphone** — audio input device selection
- **Models Directory** — where whisper models are stored
- **Models** — opens ModelModal

Auto-saves on any change (reactive `$:` block).

### ModelModal (`components/ModelModal.svelte`)

Model management dialog:
- Lists all available whisper models (tiny, base, small, medium, large)
- Shows download status and file size
- Download button with progress bar
- Delete button for downloaded models
- Listens to `model:download:progress` events

### HotkeyCapture (`components/HotkeyCapture.svelte`)

Interactive hotkey capture widget:
- Shows current hotkey binding
- Click to enter capture mode
- Captures next key combination via Go backend
- Displays human-readable key names

### ProgressBar (`components/ProgressBar.svelte`)

Reusable progress bar with percentage display.

### Toast (`components/Toast.svelte`)

Toast notification system for temporary messages.

## i18n (`lib/i18n.ts`)

Frontend translation system:
- 100+ translation keys
- 9 languages: en, ru, de, es, fr, it, pt, pl, uk
- `t(lang, key)` function for lookup
- Type-safe `Lang` type

Key translation groups:
- UI labels (settings, buttons, tooltips)
- Backend status messages
- Recording state messages
- Model management
- History

## Wails Bindings (`bindings/`)

Auto-generated TypeScript wrappers for Go service methods.

**Do NOT edit manually.** Regenerate with:
```bash
wails3 generate bindings
```

Import pattern:
```typescript
import { GetAllBackends, InstallBackend } from '../../bindings/.../settingsservice.js';
import { GetPresets, CreatePreset } from '../../bindings/.../presetservice.js';
```

## Event Handling

Frontend listens to Go events via `@wailsio/runtime`:

```typescript
import { Events } from '@wailsio/runtime';

// Subscribe
const unsub = Events.On('event-name', (event) => {
  const data = event.data?.[0] || event.data || event;
  // handle data
});

// Cleanup in onDestroy
onDestroy(() => { unsub(); });
```

## CSS Variables (Theme)

Defined in global CSS, toggled via `data-theme` attribute on `<html>`:

```css
--bg-page, --bg-overlay, --bg-input
--text-primary, --text-secondary, --text-tertiary, --text-muted
--accent, --accent-dim, --accent-red
--border-color, --border-hover
--toggle-bg, --toggle-border
```

## Design Language

- Monospace typography (`ui-monospace, monospace`)
- Steampunk/vintage aesthetic with warm accent colors
- Pill-style toggle buttons
- Minimal, functional UI with keyboard-first interaction
- Dark theme default
