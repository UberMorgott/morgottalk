#!/usr/bin/env python3
"""
Transcribation - Cross-platform speech transcription & translation tool.
Uses faster-whisper for local, offline transcription with GPU acceleration.
"""

import argparse
import json
import os
import sys
import signal
import tempfile
import time
import wave
from pathlib import Path

# ── Colors ──────────────────────────────────────────────────────────────────

class C:
    """ANSI color codes."""
    RESET   = "\033[0m"
    BOLD    = "\033[1m"
    DIM     = "\033[2m"
    RED     = "\033[91m"
    GREEN   = "\033[92m"
    YELLOW  = "\033[93m"
    BLUE    = "\033[94m"
    MAGENTA = "\033[95m"
    CYAN    = "\033[96m"
    WHITE   = "\033[97m"

    @staticmethod
    def disable():
        for attr in ["RESET","BOLD","DIM","RED","GREEN","YELLOW","BLUE","MAGENTA","CYAN","WHITE"]:
            setattr(C, attr, "")

# Disable colors if not a terminal or on Windows without ANSI support
if not sys.stdout.isatty():
    C.disable()
elif sys.platform == "win32":
    try:
        import ctypes
        kernel32 = ctypes.windll.kernel32
        kernel32.SetConsoleMode(kernel32.GetStdHandle(-11), 7)
    except Exception:
        C.disable()

# ── Helpers ─────────────────────────────────────────────────────────────────

def banner():
    print(f"""
{C.CYAN}{C.BOLD}╔══════════════════════════════════════════╗
║         🎤  Transcribation  🎤           ║
║   Speech transcription & translation     ║
╚══════════════════════════════════════════╝{C.RESET}
""")

def info(msg):
    print(f"  {C.BLUE}ℹ{C.RESET}  {msg}")

def success(msg):
    print(f"  {C.GREEN}✓{C.RESET}  {msg}")

def warn(msg):
    print(f"  {C.YELLOW}⚠{C.RESET}  {msg}")

def error(msg):
    print(f"  {C.RED}✗{C.RESET}  {msg}")

def die(msg):
    error(msg)
    sys.exit(1)

def pick(prompt, options, default=0):
    """Interactive single-choice menu. Returns index."""
    print(f"\n  {C.BOLD}{prompt}{C.RESET}")
    for i, (label, desc) in enumerate(options):
        marker = f"{C.CYAN}›{C.RESET}" if i == default else " "
        num = f"{C.DIM}[{i+1}]{C.RESET}"
        print(f"  {marker} {num} {C.BOLD}{label}{C.RESET} {C.DIM}- {desc}{C.RESET}")

    while True:
        try:
            raw = input(f"\n  {C.CYAN}>{C.RESET} Choose [1-{len(options)}] (default {default+1}): ").strip()
            if not raw:
                return default
            choice = int(raw) - 1
            if 0 <= choice < len(options):
                return choice
        except (ValueError, EOFError):
            pass
        print(f"  {C.RED}Invalid choice, try again{C.RESET}")

# ── Whisper wrapper ─────────────────────────────────────────────────────────

MODELS = [
    ("tiny",     "~75 MB,  fastest,  lowest quality"),
    ("base",     "~150 MB, fast,     decent quality"),
    ("small",    "~500 MB, balanced, good quality"),
    ("medium",   "~1.5 GB, slower,   great quality"),
    ("large-v3", "~3 GB,   slowest,  best quality"),
]

LANGUAGES = {
    "auto":  "Auto-detect",
    "en":    "English",
    "ru":    "Russian",
    "es":    "Spanish",
    "fr":    "French",
    "de":    "German",
    "zh":    "Chinese",
    "ja":    "Japanese",
    "ko":    "Korean",
    "pt":    "Portuguese",
    "it":    "Italian",
    "nl":    "Dutch",
    "pl":    "Polish",
    "uk":    "Ukrainian",
    "ar":    "Arabic",
    "hi":    "Hindi",
    "tr":    "Turkish",
    "sv":    "Swedish",
    "cs":    "Czech",
    "vi":    "Vietnamese",
    "th":    "Thai",
}

def detect_device():
    """Detect best compute device."""
    try:
        import torch
        if torch.cuda.is_available():
            gpu_name = torch.cuda.get_device_name(0)
            return "cuda", f"CUDA ({gpu_name})"
    except ImportError:
        pass

    try:
        from faster_whisper.utils import get_assets_path
        import ctranslate2
        if "cuda" in ctranslate2.get_supported_compute_types("cuda"):
            return "cuda", "CUDA"
    except Exception:
        pass

    return "cpu", "CPU"

def load_model(model_name, device):
    """Load faster-whisper model."""
    try:
        from faster_whisper import WhisperModel
    except ImportError:
        die("faster-whisper not installed. Run the install script or: pip install faster-whisper")

    compute_type = "float16" if device == "cuda" else "int8"
    info(f"Loading model {C.BOLD}{model_name}{C.RESET} on {C.BOLD}{device}{C.RESET} ({compute_type})...")

    model = WhisperModel(model_name, device=device, compute_type=compute_type)
    success("Model loaded")
    return model

def transcribe_audio(model, audio_path, language=None, translate=False):
    """Transcribe audio file, return segments."""
    task = "translate" if translate else "transcribe"
    lang = None if language == "auto" else language

    info(f"Transcribing: {C.BOLD}{audio_path}{C.RESET}")
    if translate:
        info(f"Mode: {C.YELLOW}translate to English{C.RESET}")
    if lang:
        info(f"Language: {C.BOLD}{lang}{C.RESET}")
    else:
        info("Language: auto-detect")

    segments, seg_info = model.transcribe(
        audio_path,
        language=lang,
        task=task,
        beam_size=5,
        vad_filter=True,
        vad_parameters=dict(min_silence_duration_ms=500),
    )

    detected_lang = seg_info.language
    lang_prob = seg_info.language_probability
    info(f"Detected language: {C.BOLD}{detected_lang}{C.RESET} ({lang_prob:.0%})")

    results = []
    for seg in segments:
        results.append({
            "start": seg.start,
            "end": seg.end,
            "text": seg.text.strip(),
        })
        # Print in real-time
        ts = f"{C.DIM}[{_fmt_time(seg.start)} → {_fmt_time(seg.end)}]{C.RESET}"
        print(f"  {ts} {seg.text.strip()}")

    return results, detected_lang

# ── Microphone recording ────────────────────────────────────────────────────

def record_from_mic(duration=None):
    """Record audio from microphone. Press Ctrl+C or Enter to stop."""
    try:
        import sounddevice as sd
        import numpy as np
    except ImportError:
        die("sounddevice not installed. Run: pip install sounddevice numpy")

    sample_rate = 16000
    channels = 1
    frames = []
    recording = True

    def callback(indata, frame_count, time_info, status):
        if recording:
            frames.append(indata.copy())

    info(f"Recording from microphone ({C.BOLD}{sample_rate}Hz{C.RESET})...")
    if duration:
        info(f"Duration: {duration}s")
    else:
        print(f"  {C.YELLOW}Press Enter to stop recording...{C.RESET}")

    stream = sd.InputStream(
        samplerate=sample_rate,
        channels=channels,
        dtype="float32",
        callback=callback,
    )

    with stream:
        if duration:
            # Show progress
            start_t = time.time()
            while time.time() - start_t < duration:
                elapsed = time.time() - start_t
                bar_len = 30
                filled = int(bar_len * elapsed / duration)
                bar = f"{'█' * filled}{'░' * (bar_len - filled)}"
                print(f"\r  {C.CYAN}🎙 {bar} {elapsed:.1f}s / {duration}s{C.RESET}", end="", flush=True)
                time.sleep(0.1)
            print()
        else:
            try:
                input()
            except (EOFError, KeyboardInterrupt):
                pass

    recording = False

    if not frames:
        die("No audio recorded")

    import numpy as np
    audio_data = np.concatenate(frames, axis=0)

    # Save to temp WAV file
    tmp = tempfile.NamedTemporaryFile(suffix=".wav", delete=False)
    with wave.open(tmp.name, "wb") as wf:
        wf.setnchannels(channels)
        wf.setsampwidth(2)  # 16-bit
        wf.setframerate(sample_rate)
        wf.writeframes((audio_data * 32767).astype(np.int16).tobytes())

    success(f"Recorded {len(audio_data)/sample_rate:.1f}s of audio")
    return tmp.name

# ── Output formats ──────────────────────────────────────────────────────────

def _fmt_time(seconds):
    """Format seconds as HH:MM:SS."""
    h = int(seconds // 3600)
    m = int((seconds % 3600) // 60)
    s = int(seconds % 60)
    if h > 0:
        return f"{h}:{m:02d}:{s:02d}"
    return f"{m}:{s:02d}"

def _fmt_srt_time(seconds):
    """Format seconds as SRT timestamp: HH:MM:SS,mmm"""
    h = int(seconds // 3600)
    m = int((seconds % 3600) // 60)
    s = int(seconds % 60)
    ms = int((seconds % 1) * 1000)
    return f"{h:02d}:{m:02d}:{s:02d},{ms:03d}"

def _fmt_vtt_time(seconds):
    """Format seconds as VTT timestamp: HH:MM:SS.mmm"""
    h = int(seconds // 3600)
    m = int((seconds % 3600) // 60)
    s = int(seconds % 60)
    ms = int((seconds % 1) * 1000)
    return f"{h:02d}:{m:02d}:{s:02d}.{ms:03d}"

def save_output(segments, output_path, fmt):
    """Save transcription in specified format."""
    if fmt == "txt":
        text = "\n".join(seg["text"] for seg in segments)
        Path(output_path).write_text(text, encoding="utf-8")

    elif fmt == "srt":
        lines = []
        for i, seg in enumerate(segments, 1):
            lines.append(str(i))
            lines.append(f"{_fmt_srt_time(seg['start'])} --> {_fmt_srt_time(seg['end'])}")
            lines.append(seg["text"])
            lines.append("")
        Path(output_path).write_text("\n".join(lines), encoding="utf-8")

    elif fmt == "vtt":
        lines = ["WEBVTT", ""]
        for seg in segments:
            lines.append(f"{_fmt_vtt_time(seg['start'])} --> {_fmt_vtt_time(seg['end'])}")
            lines.append(seg["text"])
            lines.append("")
        Path(output_path).write_text("\n".join(lines), encoding="utf-8")

    elif fmt == "json":
        Path(output_path).write_text(
            json.dumps(segments, ensure_ascii=False, indent=2),
            encoding="utf-8",
        )
    else:
        die(f"Unknown format: {fmt}")

    success(f"Saved: {C.BOLD}{output_path}{C.RESET} ({fmt})")

# ── Interactive mode ────────────────────────────────────────────────────────

def interactive_mode():
    """Run interactive transcription session."""
    banner()

    # Choose mode
    mode = pick("What do you want to do?", [
        ("Transcribe a file",     "Transcribe audio/video file to text"),
        ("Record from microphone","Record voice and transcribe in real-time"),
    ])

    # Choose model
    model_idx = pick("Select Whisper model:", MODELS, default=2)
    model_name = MODELS[model_idx][0]

    # Choose language
    lang_items = list(LANGUAGES.items())
    lang_idx = pick("Source language:", [(v, k) for k, v in lang_items], default=0)
    language = lang_items[lang_idx][0]

    # Translation?
    translate = False
    tr = pick("Translate to English?", [
        ("No",  "Keep original language"),
        ("Yes", "Translate everything to English"),
    ], default=0)
    translate = tr == 1

    # Output format
    fmt_idx = pick("Output format:", [
        ("txt",  "Plain text"),
        ("srt",  "SubRip subtitles"),
        ("vtt",  "WebVTT subtitles"),
        ("json", "JSON with timestamps"),
    ], default=0)
    fmt = ["txt", "srt", "vtt", "json"][fmt_idx]

    # Load model
    device, device_name = detect_device()
    info(f"Device: {C.BOLD}{device_name}{C.RESET}")
    model = load_model(model_name, device)

    # Process
    if mode == 0:
        # File transcription
        while True:
            file_path = input(f"\n  {C.CYAN}>{C.RESET} Enter file path: ").strip().strip("'\"")
            if os.path.isfile(file_path):
                break
            error("File not found, try again")

        segments, detected = transcribe_audio(model, file_path, language, translate)

        if segments:
            base = Path(file_path).stem
            out_path = f"{base}.{fmt}"
            save_output(segments, out_path, fmt)

            # Also show full text
            print(f"\n  {C.BOLD}{'─' * 50}{C.RESET}")
            full_text = " ".join(seg["text"] for seg in segments)
            print(f"  {full_text}")
            print(f"  {C.BOLD}{'─' * 50}{C.RESET}")
        else:
            warn("No speech detected")

    else:
        # Microphone recording
        dur_raw = input(f"\n  {C.CYAN}>{C.RESET} Duration in seconds (Enter for manual stop): ").strip()
        duration = float(dur_raw) if dur_raw else None

        tmp_path = record_from_mic(duration)
        try:
            segments, detected = transcribe_audio(model, tmp_path, language, translate)

            if segments:
                out_path = f"recording_{int(time.time())}.{fmt}"
                save_output(segments, out_path, fmt)

                print(f"\n  {C.BOLD}{'─' * 50}{C.RESET}")
                full_text = " ".join(seg["text"] for seg in segments)
                print(f"  {full_text}")
                print(f"  {C.BOLD}{'─' * 50}{C.RESET}")
            else:
                warn("No speech detected")
        finally:
            os.unlink(tmp_path)

    # Ask to continue
    print()
    again = pick("Continue?", [
        ("New transcription", "Start another transcription"),
        ("Exit",              "Quit the program"),
    ], default=1)

    if again == 0:
        interactive_mode()

# ── CLI ─────────────────────────────────────────────────────────────────────

def build_parser():
    parser = argparse.ArgumentParser(
        prog="transcribe",
        description="Transcribation - Speech transcription & translation tool",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=f"""
{C.BOLD}Examples:{C.RESET}
  transcribe                          # Interactive mode
  transcribe audio.mp3                # Transcribe a file
  transcribe --mic                    # Record from microphone
  transcribe --mic --duration 30      # Record 30 seconds
  transcribe --translate video.mp4    # Transcribe + translate to English
  transcribe -m large-v3 audio.wav    # Use large model
  transcribe -l ru -f srt lecture.mp3 # Russian, SRT output

{C.BOLD}Models:{C.RESET}  tiny | base | small | medium | large-v3
{C.BOLD}Formats:{C.RESET} txt | srt | vtt | json
        """,
    )

    parser.add_argument("file", nargs="?", help="Audio/video file to transcribe")
    parser.add_argument("--mic", action="store_true", help="Record from microphone")
    parser.add_argument("--duration", "-d", type=float, help="Recording duration in seconds (with --mic)")
    parser.add_argument("--model", "-m", default="small", choices=[m[0] for m in MODELS],
                        help="Whisper model size (default: small)")
    parser.add_argument("--language", "-l", default="auto", choices=list(LANGUAGES.keys()),
                        help="Source language code (default: auto-detect)")
    parser.add_argument("--translate", "-t", action="store_true",
                        help="Translate to English")
    parser.add_argument("--format", "-f", default="txt", choices=["txt","srt","vtt","json"],
                        help="Output format (default: txt)")
    parser.add_argument("--output", "-o", help="Output file path (default: auto)")
    parser.add_argument("--device", choices=["auto","cuda","cpu"], default="auto",
                        help="Compute device (default: auto)")
    parser.add_argument("--list-languages", action="store_true", help="Show all supported languages")

    return parser

def main():
    # Handle Ctrl+C gracefully
    signal.signal(signal.SIGINT, lambda *_: (print(f"\n  {C.DIM}Interrupted{C.RESET}"), sys.exit(0)))

    parser = build_parser()
    args = parser.parse_args()

    # List languages
    if args.list_languages:
        print(f"\n  {C.BOLD}Supported languages:{C.RESET}")
        for code, name in LANGUAGES.items():
            if code != "auto":
                print(f"    {C.CYAN}{code:5s}{C.RESET} {name}")
        print(f"\n  {C.DIM}Whisper supports 99 languages total.")
        print(f"  Use any ISO 639-1 code (e.g., 'fi' for Finnish).{C.RESET}\n")
        return

    # No args → interactive mode
    if not args.file and not args.mic:
        interactive_mode()
        return

    # Determine device
    if args.device == "auto":
        device, device_name = detect_device()
    elif args.device == "cuda":
        device, device_name = "cuda", "CUDA (forced)"
    else:
        device, device_name = "cpu", "CPU (forced)"

    banner()
    info(f"Device: {C.BOLD}{device_name}{C.RESET}")

    # Load model
    model = load_model(args.model, device)

    # Get audio
    tmp_path = None
    if args.mic:
        tmp_path = record_from_mic(args.duration)
        audio_path = tmp_path
    else:
        audio_path = args.file
        if not os.path.isfile(audio_path):
            die(f"File not found: {audio_path}")

    try:
        # Transcribe
        segments, detected = transcribe_audio(model, audio_path, args.language, args.translate)

        if not segments:
            warn("No speech detected in audio")
            return

        # Output
        if args.output:
            out_path = args.output
        elif args.mic:
            out_path = f"recording_{int(time.time())}.{args.format}"
        else:
            out_path = f"{Path(audio_path).stem}.{args.format}"

        save_output(segments, out_path, args.format)

        # Print summary
        print(f"\n  {C.BOLD}{'─' * 50}{C.RESET}")
        full_text = " ".join(seg["text"] for seg in segments)
        print(f"  {full_text}")
        print(f"  {C.BOLD}{'─' * 50}{C.RESET}\n")

    finally:
        if tmp_path and os.path.exists(tmp_path):
            os.unlink(tmp_path)

if __name__ == "__main__":
    main()
