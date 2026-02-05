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

# ── Key detection (cross-platform) ──────────────────────────────────────────

def _setup_raw_input():
    """Setup raw terminal input. Returns cleanup function."""
    if sys.platform == "win32":
        return lambda: None  # msvcrt doesn't need setup

    import termios, tty
    fd = sys.stdin.fileno()
    old = termios.tcgetattr(fd)
    tty.setcbreak(fd)  # cbreak mode: char-by-char, signals still work
    return lambda: termios.tcsetattr(fd, termios.TCSADRAIN, old)

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
        if "cuda" in ctranslate2.get_supported_compute_types("cuda"):
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

def run_loop(model, language, translate, mode):
    """Main push-to-talk loop."""
    print(f"  {C.DIM}{'─' * 40}{C.RESET}")
    if translate:
        info(f"Mode: transcribe + {C.YELLOW}translate to English{C.RESET}")
    if language != "auto":
        info(f"Language: {C.BOLD}{language}{C.RESET}")
    print(f"  {C.DIM}Ctrl+C to exit{C.RESET}")
    print(f"  {C.DIM}{'─' * 40}{C.RESET}\n")

    while True:
        # Record
        if mode == "hold":
            wav_path = record_push_to_talk()
        else:
            wav_path = record_toggle()

        if not wav_path:
            warn("Too short or no audio, try again")
            continue

        try:
            # Transcribe
            print(f"  {C.DIM}Transcribing...{C.RESET}", end="", flush=True)
            text, detected_lang = transcribe(model, wav_path, language, translate)
            # Clear the "Transcribing..." line
            print(f"\r                      \r", end="")

            if not text.strip():
                warn("No speech detected")
                continue

            # Show result
            print(f"\n  {C.BOLD}{C.GREEN}{text}{C.RESET}")

            # Copy to clipboard
            if copy_to_clipboard(text):
                print(f"  {C.DIM}(copied to clipboard){C.RESET}")

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
  transcribe                 # Interactive setup, then push-to-talk
  transcribe -m large-v3     # Use large model
  transcribe -t              # Translate to English
  transcribe -l ru           # Force Russian as source language
  transcribe --mode toggle   # Press Enter to start/stop instead of hold

{C.BOLD}Models:{C.RESET} tiny | base | small | medium | large-v3
        """,
    )

    parser.add_argument("--model", "-m", default=None, choices=[m[0] for m in MODELS],
                        help="Whisper model (default: interactive choice)")
    parser.add_argument("--language", "-l", default=None,
                        help="Source language code, e.g. 'ru', 'en' (default: auto)")
    parser.add_argument("--translate", "-t", action="store_true",
                        help="Translate to English")
    parser.add_argument("--mode", default=None, choices=["hold", "toggle"],
                        help="Recording mode: hold key or toggle start/stop (default: hold)")
    parser.add_argument("--device", choices=["auto", "cuda", "cpu"], default="auto",
                        help="Compute device (default: auto)")
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

    # Interactive setup if flags not provided
    model_name = args.model
    if not model_name:
        idx = pick("Select Whisper model:", MODELS, default=2)
        model_name = MODELS[idx][0]

    language = args.language or "auto"
    if args.language is None:
        lang_options = [
            ("Auto-detect", "Let Whisper detect the language"),
            ("ru — Russian", ""), ("en — English", ""),
            ("de — German", ""), ("es — Spanish", ""),
            ("fr — French", ""), ("zh — Chinese", ""),
            ("ja — Japanese", ""), ("uk — Ukrainian", ""),
        ]
        lang_codes = ["auto", "ru", "en", "de", "es", "fr", "zh", "ja", "uk"]
        idx = pick("Source language:", lang_options, default=0)
        language = lang_codes[idx]

    translate = args.translate
    if not translate:
        idx = pick("Translate to English?", [
            ("No",  "Keep original language"),
            ("Yes", "Translate everything to English"),
        ], default=0)
        translate = idx == 1

    mode = args.mode
    if not mode:
        idx = pick("Recording mode:", [
            ("Hold key", "Hold any key while speaking, release to transcribe"),
            ("Toggle",   "Press Enter to start, Enter to stop"),
        ], default=0)
        mode = "hold" if idx == 0 else "toggle"

    # Device
    if args.device == "auto":
        device, device_name = detect_device()
    elif args.device == "cuda":
        device, device_name = "cuda", "CUDA"
    else:
        device, device_name = "cpu", "CPU"

    info(f"Device: {C.BOLD}{device_name}{C.RESET}")

    # Load model
    model = load_model(model_name, device)
    print()

    # Run
    run_loop(model, language, translate, mode)

if __name__ == "__main__":
    main()
