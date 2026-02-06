#!/usr/bin/env python3
"""
Transcribation - Push-to-talk voice transcription & translation.
Hold a key to record from mic, release to transcribe. Powered by faster-whisper.
"""

import argparse
import os
import platform
import shutil
import subprocess
import sys
import signal
import tempfile
import threading
import time
import wave

# ── Colors ──────────────────────────────────────────────────────────────────

class C:
    RESET   = "\033[0m"
    BOLD    = "\033[1m"
    DIM     = "\033[2m"
    RED     = "\033[91m"
    GREEN   = "\033[92m"
    YELLOW  = "\033[93m"
    BLUE    = "\033[94m"
    CYAN    = "\033[96m"

    @staticmethod
    def disable():
        for a in ["RESET","BOLD","DIM","RED","GREEN","YELLOW","BLUE","CYAN"]:
            setattr(C, a, "")

if not sys.stdout.isatty():
    C.disable()
elif sys.platform == "win32":
    try:
        import ctypes
        ctypes.windll.kernel32.SetConsoleMode(ctypes.windll.kernel32.GetStdHandle(-11), 7)
    except Exception:
        C.disable()

# ── Helpers ─────────────────────────────────────────────────────────────────

def info(msg):    print(f"  {C.BLUE}i{C.RESET}  {msg}")
def success(msg): print(f"  {C.GREEN}+{C.RESET}  {msg}")
def warn(msg):    print(f"  {C.YELLOW}!{C.RESET}  {msg}")
def error(msg):   print(f"  {C.RED}x{C.RESET}  {msg}")
def die(msg):     error(msg); sys.exit(1)

def banner():
    print(f"""
{C.CYAN}{C.BOLD}  ╔══════════════════════════════════════╗
  ║       Transcribation                 ║
  ║   Push-to-talk voice transcription   ║
  ╚══════════════════════════════════════╝{C.RESET}
""")

def pick(prompt, options, default=0):
    """Simple numbered menu."""
    print(f"\n  {C.BOLD}{prompt}{C.RESET}")
    for i, (label, desc) in enumerate(options):
        marker = f"{C.CYAN}>{C.RESET}" if i == default else " "
        print(f"  {marker} {C.DIM}[{i+1}]{C.RESET} {C.BOLD}{label}{C.RESET} {C.DIM}- {desc}{C.RESET}")
    while True:
        try:
            raw = input(f"\n  {C.CYAN}>{C.RESET} [{default+1}]: ").strip()
            if not raw:
                return default
            n = int(raw) - 1
            if 0 <= n < len(options):
                return n
        except (ValueError, EOFError):
            pass

# ── Clipboard ───────────────────────────────────────────────────────────────

# ── Keyboard layout detection ──────────────────────────────────────────────

def detect_keyboard_layout():
    """Detect current keyboard layout. Returns language code like 'us', 'ru'."""
    # KDE Plasma (Wayland & X11) via DBus
    try:
        idx = subprocess.run(
            ["qdbus6", "org.kde.keyboard", "/Layouts",
             "org.kde.KeyboardLayouts.getLayout"],
            capture_output=True, text=True, timeout=1,
        )
        if idx.returncode == 0:
            layout_idx = int(idx.stdout.strip())
            layouts = subprocess.run(
                ["qdbus6", "--literal", "org.kde.keyboard", "/Layouts",
                 "org.kde.KeyboardLayouts.getLayoutsList"],
                capture_output=True, text=True, timeout=1,
            )
            if layouts.returncode == 0:
                # Parse: [Argument: (sss) "us", "", "...", ...]
                import re
                codes = re.findall(r'\(sss\)\s+"(\w+)"', layouts.stdout)
                if 0 <= layout_idx < len(codes):
                    return codes[layout_idx]
    except Exception:
        pass

    # GNOME via gsettings
    try:
        r = subprocess.run(
            ["gsettings", "get", "org.gnome.desktop.input-sources", "current"],
            capture_output=True, text=True, timeout=1,
        )
        if r.returncode == 0:
            idx = int(r.stdout.strip().split()[-1])
            r2 = subprocess.run(
                ["gsettings", "get", "org.gnome.desktop.input-sources", "sources"],
                capture_output=True, text=True, timeout=1,
            )
            if r2.returncode == 0:
                import re
                codes = re.findall(r"'(\w+)'\)", r2.stdout)
                if 0 <= idx < len(codes):
                    return codes[idx]
    except Exception:
        pass

    return None

def copy_to_clipboard(text):
    """Copy text to system clipboard. Silent fail if unavailable."""
    try:
        if sys.platform == "darwin":
            subprocess.run(["pbcopy"], input=text.encode(), check=True)
            return True
        elif sys.platform == "win32":
            subprocess.run(["clip.exe"], input=text.encode(), check=True)
            return True
        else:
            # Linux: try wl-copy (Wayland) then xclip (X11)
            if shutil.which("wl-copy"):
                subprocess.run(["wl-copy", text], check=True)
                return True
            elif shutil.which("xclip"):
                subprocess.run(["xclip", "-selection", "clipboard"], input=text.encode(), check=True)
                return True
            elif shutil.which("xsel"):
                subprocess.run(["xsel", "--clipboard", "--input"], input=text.encode(), check=True)
                return True
    except Exception:
        pass
    return False

def type_text(text):
    """Type text into focused window via clipboard paste (Ctrl+Shift+V)."""
    if not copy_to_clipboard(text):
        return False
    time.sleep(0.1)
    # Ctrl+Shift+V — paste in terminals (Konsole, Alacritty, etc.)
    # KEY_LEFTCTRL=29, KEY_LEFTSHIFT=42, KEY_V=47
    if shutil.which("ydotool"):
        try:
            subprocess.run(["ydotool", "key", "--key-delay", "12",
                           "29:1", "42:1", "47:1", "47:0", "42:0", "29:0"],
                           check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            return True
        except Exception:
            pass
    return False

def notify(title, body=""):
    """Show desktop notification."""
    try:
        subprocess.Popen(["notify-send", "-a", "Transcribation",
                          "-i", "audio-input-microphone",
                          "-t", "3000", title, body],
                         stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    except Exception:
        pass

# ── Key detection (cross-platform) ──────────────────────────────────────────

def _setup_raw_input():
    """Setup raw terminal input. Returns cleanup function."""
    if sys.platform == "win32":
        return lambda: None  # msvcrt doesn't need setup

    try:
        import termios, tty
        fd = sys.stdin.fileno()
        old = termios.tcgetattr(fd)
        tty.setcbreak(fd)  # cbreak mode: char-by-char, signals still work
        return lambda: termios.tcsetattr(fd, termios.TCSADRAIN, old)
    except (termios.error, OSError):
        return lambda: None  # no terminal (background/headless)

def _key_available():
    """Check if a key is available without blocking."""
    if sys.platform == "win32":
        import msvcrt
        return msvcrt.kbhit()
    else:
        import select
        return select.select([sys.stdin], [], [], 0)[0] != []

def _read_key():
    """Read a single key (blocking)."""
    if sys.platform == "win32":
        import msvcrt
        return msvcrt.getwch()
    else:
        return sys.stdin.read(1)

def _drain_keys():
    """Drain any buffered key-repeat events."""
    while _key_available():
        _read_key()

# ── evdev key detection (Linux, keyboard hot-plug) ────────────────────────

def _check_evdev():
    """Check if evdev is usable for direct key press/release detection."""
    if sys.platform != "linux":
        return False
    try:
        import evdev
        paths = evdev.list_devices()
        if not paths:
            return False
        dev = evdev.InputDevice(paths[0])
        dev.close()
        return True
    except (ImportError, PermissionError, OSError):
        return False

def _evdev_hint():
    """Print hint if evdev could work but permissions are missing."""
    try:
        import evdev
        paths = evdev.list_devices()
        if paths:
            try:
                dev = evdev.InputDevice(paths[0])
                dev.close()
            except PermissionError:
                warn("Keyboard hot-plug: add yourself to 'input' group:")
                print(f"  {C.DIM}    sudo usermod -aG input $USER  (then re-login){C.RESET}")
    except ImportError:
        pass

def _open_keyboards(drain=False):
    """Find and open all keyboard input devices via evdev."""
    import evdev
    from evdev import ecodes
    keyboards = []
    for path in evdev.list_devices():
        try:
            dev = evdev.InputDevice(path)
            caps = dev.capabilities()
            if ecodes.EV_KEY in caps:
                keys = caps[ecodes.EV_KEY]
                if any(k in keys for k in (ecodes.KEY_A, ecodes.KEY_SPACE, ecodes.KEY_ENTER)):
                    if drain:
                        # Flush any pending events so we don't pick up old keypresses
                        try:
                            while dev.read_one():
                                pass
                        except Exception:
                            pass
                    keyboards.append(dev)
                else:
                    dev.close()
            else:
                dev.close()
        except (PermissionError, OSError):
            continue
    return keyboards

def _close_devices(devices):
    """Close evdev input devices."""
    for d in devices:
        try:
            d.close()
        except Exception:
            pass

# ── Recording ───────────────────────────────────────────────────────────────

def record_push_to_talk():
    """Hold-to-record: hold any key, release to stop. Returns temp WAV path."""
    try:
        import sounddevice as sd
        import numpy as np
    except ImportError:
        die("sounddevice/numpy not installed. Run the install script.")

    sample_rate = 16000
    frames = []
    recording = True

    def audio_callback(indata, frame_count, time_info, status):
        if recording:
            frames.append(indata.copy())

    stream = sd.InputStream(samplerate=sample_rate, channels=1, dtype="float32",
                            callback=audio_callback)
    cleanup = _setup_raw_input()

    try:
        # Wait for key press to start
        print(f"  {C.YELLOW}>> Press and hold any key to record...{C.RESET}", flush=True)
        _read_key()
        _drain_keys()

        # Start recording
        stream.start()
        start_time = time.time()
        print(f"\r  {C.RED}>> RECORDING  ", end="", flush=True)

        # Record while key is held (key-repeat sends events)
        # Stop when no key event comes for a short timeout
        last_key_time = time.time()
        while True:
            if _key_available():
                _read_key()
                last_key_time = time.time()

            elapsed = time.time() - start_time
            # Show timer
            print(f"\r  {C.RED}>> RECORDING  {elapsed:.1f}s{C.RESET}  ", end="", flush=True)

            # If no key for 0.25s, consider released
            if time.time() - last_key_time > 0.25:
                break

            time.sleep(0.02)

        stream.stop()
        recording = False
        elapsed = time.time() - start_time
        print(f"\r  {C.GREEN}>> Recorded {elapsed:.1f}s{C.RESET}              ")

    finally:
        cleanup()
        stream.close()

    if not frames:
        return None

    import numpy as np
    audio = np.concatenate(frames, axis=0)
    if len(audio) / sample_rate < 0.3:
        return None  # Too short

    tmp = tempfile.NamedTemporaryFile(suffix=".wav", delete=False)
    with wave.open(tmp.name, "wb") as wf:
        wf.setnchannels(1)
        wf.setsampwidth(2)
        wf.setframerate(sample_rate)
        wf.writeframes((audio * 32767).astype(np.int16).tobytes())

    return tmp.name

def record_push_to_talk_evdev(trigger_key=None):
    """Hold-to-record using evdev. Detects actual key press/release, supports keyboard hot-plug."""
    from evdev import ecodes
    import select

    try:
        import sounddevice as sd
        import numpy as np
    except ImportError:
        die("sounddevice/numpy not installed. Run the install script.")

    # Default trigger: Right Ctrl
    if trigger_key is None:
        trigger_key = ecodes.KEY_RIGHTCTRL

    sample_rate = 16000
    frames = []
    rec_flag = True

    def audio_cb(indata, *_):
        if rec_flag:
            frames.append(indata.copy())

    stream = sd.InputStream(samplerate=sample_rate, channels=1, dtype="float32",
                            callback=audio_cb)
    keyboards = _open_keyboards(drain=True)
    cleanup = _setup_raw_input()

    try:
        key_name = ecodes.KEY.get(trigger_key, str(trigger_key))
        print(f"  {C.YELLOW}>> Hold {C.BOLD}{key_name}{C.YELLOW} to record...{C.RESET}", flush=True)

        # Wait for trigger key press (re-scans each second for hot-plugged keyboards)
        pressed = False
        while not pressed:
            if not keyboards:
                time.sleep(0.5)
                keyboards = _open_keyboards()
                continue

            try:
                readable, _, _ = select.select(keyboards, [], [], 1.0)
            except (ValueError, OSError):
                _close_devices(keyboards)
                keyboards = _open_keyboards()
                continue

            if not readable:
                # Timeout — re-scan for hot-plugged keyboards
                _close_devices(keyboards)
                keyboards = _open_keyboards()
                continue

            for dev in readable:
                try:
                    for ev in dev.read():
                        if ev.type == ecodes.EV_KEY and ev.value == 1 and ev.code == trigger_key:
                            pressed = True
                            break
                except (OSError, IOError):
                    try:
                        dev.close()
                    except Exception:
                        pass
                    keyboards = [kb for kb in keyboards if kb is not dev]
                if pressed:
                    break

        # Record until trigger key released
        stream.start()
        start_time = time.time()
        released = False

        while not released:
            try:
                readable, _, _ = select.select(keyboards, [], [], 0.05)
            except (ValueError, OSError):
                break

            for dev in readable:
                try:
                    for ev in dev.read():
                        if ev.type == ecodes.EV_KEY and ev.code == trigger_key and ev.value == 0:
                            released = True
                            break
                except (OSError, IOError):
                    pass
                if released:
                    break

            elapsed = time.time() - start_time
            print(f"\r  {C.RED}>> RECORDING  {elapsed:.1f}s{C.RESET}  ", end="", flush=True)

        stream.stop()
        rec_flag = False
        elapsed = time.time() - start_time
        print(f"\r  {C.GREEN}>> Recorded {elapsed:.1f}s{C.RESET}              ")

    finally:
        _drain_keys()
        cleanup()
        stream.close()
        _close_devices(keyboards)

    if not frames:
        return None

    audio = np.concatenate(frames, axis=0)
    if len(audio) / sample_rate < 0.3:
        return None

    tmp = tempfile.NamedTemporaryFile(suffix=".wav", delete=False)
    with wave.open(tmp.name, "wb") as wf:
        wf.setnchannels(1)
        wf.setsampwidth(2)
        wf.setframerate(sample_rate)
        wf.writeframes((audio * 32767).astype(np.int16).tobytes())

    return tmp.name

def record_toggle():
    """Press Enter to start, Enter to stop. Fallback mode. Returns temp WAV path."""
    try:
        import sounddevice as sd
        import numpy as np
    except ImportError:
        die("sounddevice/numpy not installed. Run the install script.")

    sample_rate = 16000
    frames = []
    recording = True

    def audio_callback(indata, frame_count, time_info, status):
        if recording:
            frames.append(indata.copy())

    print(f"  {C.YELLOW}>> Press Enter to START recording{C.RESET}", flush=True)
    input()

    stream = sd.InputStream(samplerate=sample_rate, channels=1, dtype="float32",
                            callback=audio_callback)
    stream.start()
    start_time = time.time()

    # Show timer in background
    stop_flag = threading.Event()

    def show_timer():
        while not stop_flag.is_set():
            e = time.time() - start_time
            print(f"\r  {C.RED}>> RECORDING  {e:.1f}s{C.RESET}  ", end="", flush=True)
            time.sleep(0.1)

    timer_thread = threading.Thread(target=show_timer, daemon=True)
    timer_thread.start()

    print(f"\r  {C.YELLOW}>> Press Enter to STOP{C.RESET}              ", flush=True)
    input()

    stop_flag.set()
    stream.stop()
    recording = False
    stream.close()

    elapsed = time.time() - start_time
    print(f"\r  {C.GREEN}>> Recorded {elapsed:.1f}s{C.RESET}              ")

    if not frames:
        return None

    import numpy as np
    audio = np.concatenate(frames, axis=0)
    if len(audio) / sample_rate < 0.3:
        return None

    tmp = tempfile.NamedTemporaryFile(suffix=".wav", delete=False)
    with wave.open(tmp.name, "wb") as wf:
        wf.setnchannels(1)
        wf.setsampwidth(2)
        wf.setframerate(sample_rate)
        wf.writeframes((audio * 32767).astype(np.int16).tobytes())

    return tmp.name

# ── Whisper ─────────────────────────────────────────────────────────────────

MODELS = [
    ("tiny",     "~75 MB,  fastest,  lower quality"),
    ("base",     "~150 MB, fast,     decent quality"),
    ("small",    "~500 MB, balanced, good quality"),
    ("medium",   "~1.5 GB, slower,   great quality"),
    ("large-v3", "~3 GB,   slowest,  best quality"),
]

def detect_device():
    try:
        import ctranslate2
        types = ctranslate2.get_supported_compute_types("cuda")
        if types:  # non-empty means CUDA is available
            return "cuda", "CUDA"
    except Exception:
        pass
    try:
        import torch
        if torch.cuda.is_available():
            return "cuda", f"CUDA ({torch.cuda.get_device_name(0)})"
    except ImportError:
        pass
    return "cpu", "CPU"

def load_model(model_name, device):
    try:
        from faster_whisper import WhisperModel
    except ImportError:
        die("faster-whisper not installed. Run the install script.")

    compute_type = "float16" if device == "cuda" else "int8"
    info(f"Loading model {C.BOLD}{model_name}{C.RESET} ({device}, {compute_type})...")
    model = WhisperModel(model_name, device=device, compute_type=compute_type)
    success("Model ready")
    return model

def transcribe(model, audio_path, language=None, translate=False):
    """Transcribe audio, return text string."""
    task = "translate" if translate else "transcribe"
    lang = None if language == "auto" else language

    segments, seg_info = model.transcribe(
        audio_path,
        language=lang,
        task=task,
        beam_size=5,
        vad_filter=True,
        vad_parameters=dict(min_silence_duration_ms=500),
    )

    parts = []
    for seg in segments:
        parts.append(seg.text.strip())

    text = " ".join(parts)
    return text, seg_info.language

# ── Main loop ───────────────────────────────────────────────────────────────

KEY_ALIASES = {
    "rctrl": "KEY_RIGHTCTRL", "rightctrl": "KEY_RIGHTCTRL",
    "lctrl": "KEY_LEFTCTRL", "leftctrl": "KEY_LEFTCTRL",
    "ralt": "KEY_RIGHTALT", "rightalt": "KEY_RIGHTALT",
    "lalt": "KEY_LEFTALT", "leftalt": "KEY_LEFTALT",
    "rshift": "KEY_RIGHTSHIFT", "rightshift": "KEY_RIGHTSHIFT",
    "lshift": "KEY_LEFTSHIFT", "leftshift": "KEY_LEFTSHIFT",
    "space": "KEY_SPACE", "pause": "KEY_PAUSE",
    "scrolllock": "KEY_SCROLLLOCK", "capslock": "KEY_CAPSLOCK",
    "insert": "KEY_INSERT", "f13": "KEY_F13", "f14": "KEY_F14",
}

def resolve_key(name):
    """Resolve a key name to evdev keycode."""
    from evdev import ecodes
    if name is None:
        return None  # use default (Right Ctrl)
    name_lower = name.lower().strip()
    ecodes_name = KEY_ALIASES.get(name_lower, f"KEY_{name.upper()}")
    code = getattr(ecodes, ecodes_name, None)
    if code is None:
        die(f"Unknown key: {name!r}. Use --key rctrl, space, pause, etc.")
    return code

LAYOUT_LANG = {
    "us": ("en", True),   # English layout → translate everything to English
    "ru": ("ru", False),  # Russian layout → transcribe as Russian
}

def run_loop(model, language, translate, mode, type_mode=False, trigger_key=None):
    """Main push-to-talk loop."""
    is_tty = sys.stdout.isatty()
    auto_layout = (language == "auto" and not translate)

    if is_tty:
        print(f"  {C.DIM}{'─' * 40}{C.RESET}")
        if auto_layout:
            info(f"Output language: {C.BOLD}follows keyboard layout{C.RESET}")
        elif translate:
            info(f"Mode: transcribe + {C.YELLOW}translate to English{C.RESET}")
        if language != "auto":
            info(f"Language: {C.BOLD}{language}{C.RESET}")

    use_evdev = mode == "hold" and _check_evdev()
    if is_tty:
        if use_evdev:
            info(f"Key detection: {C.BOLD}evdev{C.RESET} (keyboard hot-plug supported)")
        elif mode == "hold" and sys.platform == "linux":
            _evdev_hint()
        print(f"  {C.DIM}Ctrl+C to exit{C.RESET}")
        print(f"  {C.DIM}{'─' * 40}{C.RESET}\n")

    if not is_tty and type_mode:
        notify("Transcribation", "Push-to-talk ready — hold key to record")

    while True:
        # Record
        if mode == "hold":
            if use_evdev:
                wav_path = record_push_to_talk_evdev(trigger_key=trigger_key)
            else:
                wav_path = record_push_to_talk()
        else:
            wav_path = record_toggle()

        if not wav_path:
            if is_tty:
                warn("Too short or no audio, try again")
            continue

        try:
            # Detect layout → pick language & task
            cur_lang = language
            cur_translate = translate
            if auto_layout:
                layout = detect_keyboard_layout()
                if layout and layout in LAYOUT_LANG:
                    cur_lang, cur_translate = LAYOUT_LANG[layout]

            # Transcribe
            if is_tty:
                label = f"→ EN" if cur_translate else (f"→ {cur_lang.upper()}" if cur_lang != "auto" else "")
                print(f"  {C.DIM}Transcribing {label}...{C.RESET}", end="", flush=True)
            text, detected_lang = transcribe(model, wav_path, cur_lang, cur_translate)
            if is_tty:
                print(f"\r                      \r", end="")

            if not text.strip():
                if is_tty:
                    warn("No speech detected")
                continue

            # Type into focused window
            if type_mode:
                type_text(text)

            # Show result
            if is_tty:
                print(f"\n  {C.BOLD}{C.GREEN}{text}{C.RESET}")

            # Copy to clipboard
            if copy_to_clipboard(text):
                if is_tty:
                    print(f"  {C.DIM}(copied to clipboard){C.RESET}")

            if not is_tty:
                notify("Transcribed", text[:200])

            if is_tty:
                print()

        finally:
            if os.path.exists(wav_path):
                os.unlink(wav_path)

# ── CLI ─────────────────────────────────────────────────────────────────────

def main():
    signal.signal(signal.SIGINT, lambda *_: (print(f"\n\n  {C.DIM}Bye!{C.RESET}\n"), sys.exit(0)))

    parser = argparse.ArgumentParser(
        prog="transcribe",
        description="Push-to-talk voice transcription & translation",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=f"""
{C.BOLD}Examples:{C.RESET}
  transcribe                 # Auto-detect language, push-to-talk (Right Ctrl)
  transcribe -m large-v3     # Use large model
  transcribe -t              # Translate everything to English
  transcribe -l ru           # Force Russian as source language
  transcribe -k space        # Use Space as push-to-talk key
  transcribe --mode toggle   # Press Enter to start/stop instead of hold

{C.BOLD}Models:{C.RESET} tiny | base | small | medium | large-v3
        """,
    )

    parser.add_argument("--model", "-m", default="small", choices=[m[0] for m in MODELS],
                        help="Whisper model (default: small)")
    parser.add_argument("--language", "-l", default="auto",
                        help="Source language code, e.g. 'ru', 'en' (default: auto)")
    parser.add_argument("--translate", "-t", action="store_true",
                        help="Translate to English")
    parser.add_argument("--mode", default="hold", choices=["hold", "toggle"],
                        help="Recording mode: hold key or toggle start/stop (default: hold)")
    parser.add_argument("--device", choices=["auto", "cuda", "cpu"], default="auto",
                        help="Compute device (default: auto)")
    parser.add_argument("--type", dest="type_text", action="store_true",
                        help="Type transcribed text into focused window (ydotool/wtype)")
    parser.add_argument("--key", "-k", default=None,
                        help="Push-to-talk key for evdev mode (default: rctrl). "
                             "Examples: rctrl, lctrl, space, ralt, pause")
    parser.add_argument("--list-languages", action="store_true",
                        help="Show supported language codes")

    args = parser.parse_args()

    if args.list_languages:
        print(f"\n  {C.BOLD}Common language codes:{C.RESET}")
        for code, name in [
            ("auto","Auto-detect"), ("en","English"), ("ru","Russian"), ("es","Spanish"),
            ("fr","French"), ("de","German"), ("zh","Chinese"), ("ja","Japanese"),
            ("ko","Korean"), ("pt","Portuguese"), ("it","Italian"), ("uk","Ukrainian"),
            ("ar","Arabic"), ("hi","Hindi"), ("tr","Turkish"), ("pl","Polish"),
        ]:
            print(f"    {C.CYAN}{code:5s}{C.RESET} {name}")
        print(f"\n  {C.DIM}Whisper supports 99 languages. Use any ISO 639-1 code.{C.RESET}\n")
        return

    banner()

    model_name = args.model
    language = args.language
    translate = args.translate
    mode = args.mode

    # Device
    if args.device == "auto":
        device, device_name = detect_device()
    elif args.device == "cuda":
        device, device_name = "cuda", "CUDA"
    else:
        device, device_name = "cpu", "CPU"

    info(f"Device: {C.BOLD}{device_name}{C.RESET}")

    # Load model
    if sys.stdout.isatty():
        model = load_model(model_name, device)
        print()
    else:
        model = load_model(model_name, device)

    # Resolve push-to-talk key
    trigger_key = resolve_key(args.key)

    # Run
    run_loop(model, language, translate, mode, type_mode=args.type_text, trigger_key=trigger_key)

if __name__ == "__main__":
    main()
