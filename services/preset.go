package services

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/UberMorgott/transcribation/internal/config"
)

// PresetState represents the recording state of a preset.
type PresetState struct {
	ID    string `json:"id"`
	State string `json:"state"` // "idle", "recording", "processing"
}

// PresetService manages presets, recording, and transcription.
type PresetService struct {
	mu       sync.Mutex
	cfg      *config.AppConfig
	engines  map[string]*WhisperEngine // preset ID → loaded engine
	audio    *AudioCapture
	history  *HistoryService
	models   *ModelService
	hotkeys  *HotkeyManager
	states   map[string]string // preset ID → "idle"/"recording"/"processing"
	lastText string
}

func NewPresetService(history *HistoryService, models *ModelService) *PresetService {
	cfg, _ := config.Load()
	return &PresetService{
		cfg:     cfg,
		engines: make(map[string]*WhisperEngine),
		history: history,
		models:  models,
		states:  make(map[string]string),
	}
}

// Init initializes audio and registers hotkeys for enabled presets.
func (s *PresetService) Init() error {
	audio, err := NewAudioCapture()
	if err != nil {
		return fmt.Errorf("audio init: %w", err)
	}
	if s.cfg.MicrophoneID != "" {
		audio.SetMicrophoneID(s.cfg.MicrophoneID)
	}
	s.audio = audio

	s.hotkeys = NewHotkeyManager(
		func(presetID string) { s.onHotkeyPress(presetID) },
		func(presetID string) { s.onHotkeyRelease(presetID) },
	)
	s.hotkeys.Start()

	// Register hotkeys for enabled presets and preload models if keepModelLoaded
	for i := range s.cfg.Presets {
		p := &s.cfg.Presets[i]
		s.states[p.ID] = "idle"
		if p.Enabled {
			s.activatePreset(p)
		}
	}

	return nil
}

// activatePreset registers hotkey and optionally preloads model.
// Must be called WITHOUT s.mu held (hotkey.Register and model loading can block).
func (s *PresetService) activatePreset(p *config.Preset) {
	if p.Hotkey != "" && s.hotkeys != nil {
		if err := s.hotkeys.Register(p.ID, p.Hotkey, p.InputMode); err != nil {
			log.Printf("Failed to register hotkey for preset %q: %v", p.Name, err)
		}
	}
	if p.KeepModelLoaded {
		if _, err := s.getOrLoadEngine(p); err != nil {
			log.Printf("Failed to preload model for preset %q: %v", p.Name, err)
		}
	}
}

// deactivatePreset unregisters hotkey and unloads model.
// Must be called WITHOUT s.mu held.
func (s *PresetService) deactivatePreset(presetID string) {
	if s.hotkeys != nil {
		s.hotkeys.Unregister(presetID)
	}
	s.mu.Lock()
	if engine, ok := s.engines[presetID]; ok {
		engine.Close()
		delete(s.engines, presetID)
	}
	s.mu.Unlock()
}

func (s *PresetService) onHotkeyPress(presetID string) {
	s.mu.Lock()
	p := s.findPresetByID(presetID)
	if p == nil {
		log.Printf("onHotkeyPress: preset %s not found", presetID)
		s.mu.Unlock()
		return
	}
	mode := p.InputMode
	s.mu.Unlock()

	log.Printf("onHotkeyPress: preset=%s mode=%s", presetID, mode)

	switch mode {
	case "hold":
		if err := s.StartRecording(presetID); err != nil {
			log.Printf("StartRecording failed: %v", err)
		}
	case "toggle":
		s.mu.Lock()
		state := s.states[presetID]
		s.mu.Unlock()
		if state == "recording" {
			if _, err := s.StopRecording(presetID); err != nil {
				log.Printf("StopRecording failed: %v", err)
			}
		} else {
			if err := s.StartRecording(presetID); err != nil {
				log.Printf("StartRecording failed: %v", err)
			}
		}
	}
}

func (s *PresetService) onHotkeyRelease(presetID string) {
	s.mu.Lock()
	p := s.findPresetByID(presetID)
	if p == nil {
		s.mu.Unlock()
		return
	}
	mode := p.InputMode
	state := s.states[presetID]
	s.mu.Unlock()

	log.Printf("onHotkeyRelease: preset=%s mode=%s state=%s", presetID, mode, state)

	if mode == "hold" && state == "recording" {
		if _, err := s.StopRecording(presetID); err != nil {
			log.Printf("StopRecording failed: %v", err)
		}
	}
}

// GetPresets returns all presets.
func (s *PresetService) GetPresets() []config.Preset {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.Presets
}

// CreatePreset adds a new preset and saves config.
func (s *PresetService) CreatePreset(p config.Preset) config.Preset {
	s.mu.Lock()
	defer s.mu.Unlock()

	p.ID = uuid.New().String()
	if p.InputMode == "" {
		p.InputMode = "hold"
	}
	if p.Language == "" {
		p.Language = "auto"
	}
	s.cfg.Presets = append(s.cfg.Presets, p)
	s.states[p.ID] = "idle"
	_ = config.Save(s.cfg)
	return p
}

// UpdatePreset updates a preset and re-registers hotkeys/models only when needed.
func (s *PresetService) UpdatePreset(p config.Preset) error {
	s.mu.Lock()
	idx := s.findPresetIndex(p.ID)
	if idx < 0 {
		s.mu.Unlock()
		return fmt.Errorf("preset not found: %s", p.ID)
	}

	old := s.cfg.Presets[idx]
	s.cfg.Presets[idx] = p
	_ = config.Save(s.cfg)
	s.mu.Unlock()

	// Only re-register if hotkey-related or model-related fields changed
	hotkeyChanged := old.Hotkey != p.Hotkey || old.InputMode != p.InputMode || old.Enabled != p.Enabled
	modelChanged := old.ModelName != p.ModelName || old.KeepModelLoaded != p.KeepModelLoaded

	if hotkeyChanged || modelChanged {
		go func() {
			if old.Enabled {
				s.deactivatePreset(p.ID)
			}
			if p.Enabled {
				s.activatePreset(&p)
			}
		}()
	}

	return nil
}

// DeletePreset removes a preset.
func (s *PresetService) DeletePreset(id string) error {
	s.mu.Lock()
	idx := s.findPresetIndex(id)
	if idx < 0 {
		s.mu.Unlock()
		return fmt.Errorf("preset not found: %s", id)
	}

	delete(s.states, id)
	s.cfg.Presets = append(s.cfg.Presets[:idx], s.cfg.Presets[idx+1:]...)
	_ = config.Save(s.cfg)
	s.mu.Unlock()

	go s.deactivatePreset(id)
	return nil
}

// SetPresetEnabled enables or disables a preset (hotkey + model preloading).
func (s *PresetService) SetPresetEnabled(id string, enabled bool) error {
	s.mu.Lock()
	idx := s.findPresetIndex(id)
	if idx < 0 {
		s.mu.Unlock()
		return fmt.Errorf("preset not found: %s", id)
	}

	s.cfg.Presets[idx].Enabled = enabled
	p := s.cfg.Presets[idx] // copy
	_ = config.Save(s.cfg)
	s.mu.Unlock()

	// Run in background — hotkey.Register and model loading can block for seconds
	go func() {
		if enabled {
			s.activatePreset(&p)
		} else {
			s.deactivatePreset(id)
		}
	}()

	return nil
}

// ReorderPresets reorders presets to match the given ID order.
func (s *PresetService) ReorderPresets(ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(ids) != len(s.cfg.Presets) {
		return fmt.Errorf("id count mismatch: got %d, have %d", len(ids), len(s.cfg.Presets))
	}

	byID := make(map[string]config.Preset, len(s.cfg.Presets))
	for _, p := range s.cfg.Presets {
		byID[p.ID] = p
	}

	reordered := make([]config.Preset, 0, len(ids))
	for _, id := range ids {
		p, ok := byID[id]
		if !ok {
			return fmt.Errorf("unknown preset id: %s", id)
		}
		reordered = append(reordered, p)
	}

	s.cfg.Presets = reordered
	return config.Save(s.cfg)
}

// StartRecording begins audio capture for a preset.
func (s *PresetService) StartRecording(presetID string) error {
	s.mu.Lock()

	// Check if any preset is already recording
	for id, state := range s.states {
		if state == "recording" && id != presetID {
			s.mu.Unlock()
			return fmt.Errorf("preset %s is already recording", id)
		}
	}

	p := s.findPresetByID(presetID)
	if p == nil {
		s.mu.Unlock()
		return fmt.Errorf("preset not found: %s", presetID)
	}

	if s.audio == nil {
		s.mu.Unlock()
		return fmt.Errorf("audio not initialized")
	}

	s.mu.Unlock()

	// Start audio outside lock — can block on device open
	if err := s.audio.Start(); err != nil {
		return err
	}

	s.mu.Lock()
	s.states[presetID] = "recording"
	s.mu.Unlock()
	return nil
}

// StopRecording stops capture and returns transcribed text.
func (s *PresetService) StopRecording(presetID string) (string, error) {
	s.mu.Lock()
	if s.states[presetID] != "recording" {
		s.mu.Unlock()
		return "", nil
	}

	samples := s.audio.Stop()
	s.states[presetID] = "processing"
	p := s.findPresetByID(presetID)
	if p == nil {
		s.states[presetID] = "idle"
		s.mu.Unlock()
		return "", fmt.Errorf("preset not found")
	}
	preset := *p // copy
	s.mu.Unlock()

	// Minimum recording duration: 0.5s at 16kHz = 8000 samples.
	// Short accidental presses produce silence that whisper hallucinates on.
	const minSamples = 8000
	if len(samples) < minSamples {
		log.Printf("Recording too short (%d samples, need %d), discarding", len(samples), minSamples)
		s.mu.Lock()
		s.states[presetID] = "idle"
		s.mu.Unlock()
		return "", nil
	}

	engine, err := s.getOrLoadEngine(&preset)
	if err != nil {
		s.mu.Lock()
		s.states[presetID] = "idle"
		s.mu.Unlock()
		return "", fmt.Errorf("model load failed: %w", err)
	}

	lang := preset.Language
	translate := false
	if lang == "" {
		lang = "auto"
	}

	// Override language with keyboard layout if enabled
	if preset.UseKBLayout {
		if detected := detectKeyboardLanguage(); detected != "" {
			log.Printf("KB layout detected language: %s", detected)
			lang = detected
		}
	}

	text, err := engine.TranscribeLong(samples, lang, translate)
	if err != nil {
		log.Printf("Transcription error: %v", err)
	}

	result := strings.TrimSpace(text)

	// Filter out whisper hallucinations on silence/short audio
	if isHallucination(result) {
		log.Printf("Filtered hallucination: %q", result)
		result = ""
	}

	if result != "" {
		// Paste into active text field
		if err := pasteText(result); err != nil {
			log.Printf("Paste failed: %v", err)
		}

		if preset.KeepHistory && s.history != nil {
			_ = s.history.AddEntry(result, lang)
		}
	}

	// Unload model if not keeping it loaded
	s.mu.Lock()
	if !preset.KeepModelLoaded {
		if e, ok := s.engines[presetID]; ok {
			e.Close()
			delete(s.engines, presetID)
		}
	}
	s.states[presetID] = "idle"
	s.lastText = result
	s.mu.Unlock()

	return result, nil
}

// GetRecordingStates returns the state of all presets.
func (s *PresetService) GetRecordingStates() []PresetState {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []PresetState
	for _, p := range s.cfg.Presets {
		state := s.states[p.ID]
		if state == "" {
			state = "idle"
		}
		result = append(result, PresetState{ID: p.ID, State: state})
	}
	return result
}

// GetLastText returns the last transcription result.
func (s *PresetService) GetLastText() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastText
}

// CaptureHotkey blocks until the user presses a key/combo and returns it.
func (s *PresetService) CaptureHotkey() string {
	if s.hotkeys == nil {
		return ""
	}
	return s.hotkeys.CaptureHotkey()
}

// CancelCapture cancels an in-progress key capture.
func (s *PresetService) CancelCapture() {
	if s.hotkeys != nil {
		s.hotkeys.CancelCapture()
	}
}

// GetModelLanguages returns available languages for a specific model.
func (s *PresetService) GetModelLanguages(modelName string) []LanguageInfo {
	if isEnglishOnlyModel(modelName) {
		return []LanguageInfo{{"en", "English"}}
	}
	return allLanguages()
}

func isEnglishOnlyModel(name string) bool {
	// Models with ".en" suffix are English-only
	parts := strings.Split(name, "-")
	for _, p := range parts {
		if strings.HasSuffix(p, ".en") {
			return true
		}
	}
	return strings.HasSuffix(name, ".en")
}

func allLanguages() []LanguageInfo {
	return []LanguageInfo{
		{"auto", "Auto-detect"},
		{"en", "English"},
		{"ru", "Russian"},
		{"zh", "Chinese"},
		{"de", "German"},
		{"es", "Spanish"},
		{"fr", "French"},
		{"it", "Italian"},
		{"ja", "Japanese"},
		{"ko", "Korean"},
		{"nl", "Dutch"},
		{"pl", "Polish"},
		{"pt", "Portuguese"},
		{"tr", "Turkish"},
		{"uk", "Ukrainian"},
		{"ar", "Arabic"},
		{"cs", "Czech"},
		{"da", "Danish"},
		{"fi", "Finnish"},
		{"el", "Greek"},
		{"he", "Hebrew"},
		{"hi", "Hindi"},
		{"hu", "Hungarian"},
		{"id", "Indonesian"},
		{"ms", "Malay"},
		{"no", "Norwegian"},
		{"ro", "Romanian"},
		{"sk", "Slovak"},
		{"sv", "Swedish"},
		{"th", "Thai"},
		{"vi", "Vietnamese"},
	}
}

// Shutdown releases all resources.
func (s *PresetService) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.hotkeys != nil {
		s.hotkeys.Stop()
	}
	for id, engine := range s.engines {
		engine.Close()
		delete(s.engines, id)
	}
	if s.audio != nil {
		s.audio.Close()
	}
}

// getOrLoadEngine returns a cached engine or loads a new one.
func (s *PresetService) getOrLoadEngine(p *config.Preset) (*WhisperEngine, error) {
	s.mu.Lock()
	if engine, ok := s.engines[p.ID]; ok {
		s.mu.Unlock()
		return engine, nil
	}
	s.mu.Unlock()

	modelPath, err := s.findModel(p.ModelName)
	if err != nil {
		return nil, err
	}

	backend := s.cfg.Backend
	if backend == "" {
		backend = "auto"
	}

	log.Printf("Loading whisper model for preset %q: %s (backend: %s)", p.Name, modelPath, backend)
	engine, err := NewWhisperEngine(modelPath, backend)
	if err != nil {
		return nil, fmt.Errorf("whisper init: %w", err)
	}

	s.mu.Lock()
	s.engines[p.ID] = engine
	s.mu.Unlock()

	log.Printf("Model loaded for preset %q", p.Name)
	return engine, nil
}

func (s *PresetService) findModel(modelName string) (string, error) {
	dir := s.models.ResolveModelsDir()

	fileName := "ggml-" + modelName + ".bin"
	path := filepath.Join(dir, fileName)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("cannot read models dir %s: %w", dir, err)
	}

	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".bin") {
			return filepath.Join(dir, e.Name()), nil
		}
	}

	return "", fmt.Errorf("no model found in %s (looking for %s)", dir, modelName)
}

// isHallucination detects common whisper hallucinations produced on silence.
func isHallucination(text string) bool {
	if text == "" {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(text))

	// Pure punctuation / ellipsis / musical notes
	cleaned := strings.Map(func(r rune) rune {
		if r == '.' || r == ',' || r == '!' || r == '?' || r == '-' ||
			r == '…' || r == ' ' || r == '\n' || r == '\t' ||
			r == '♪' || r == '♫' || r == '🎵' || r == '*' {
			return -1
		}
		return r
	}, lower)
	if cleaned == "" {
		return true
	}

	// Known hallucination phrases (whisper on silence)
	hallucinations := []string{
		"продолжение следует",
		"субтитры сделал",
		"субтитры делал",
		"субтитры создан",
		"спасибо за просмотр",
		"спасибо за внимание",
		"подписывайтесь на канал",
		"до свидания",
		"до новых встреч",
		"благодарю за внимание",
		"редактор субтитров",
		"thank you",
		"thanks for watching",
		"subscribe",
		"like and subscribe",
		"please subscribe",
		"the end",
		"to be continued",
		"subtitles by",
		"translated by",
		"you",
		"bye",
	}
	for _, h := range hallucinations {
		if strings.Contains(lower, h) {
			return true
		}
	}

	// Very short output (1-2 words) that's just filler
	if len([]rune(cleaned)) <= 3 {
		return true
	}

	return false
}

func (s *PresetService) findPresetByID(id string) *config.Preset {
	for i := range s.cfg.Presets {
		if s.cfg.Presets[i].ID == id {
			return &s.cfg.Presets[i]
		}
	}
	return nil
}

func (s *PresetService) findPresetIndex(id string) int {
	for i := range s.cfg.Presets {
		if s.cfg.Presets[i].ID == id {
			return i
		}
	}
	return -1
}
