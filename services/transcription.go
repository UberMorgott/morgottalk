package services

// TranscriptionService handles recording, transcription, and text output.
type TranscriptionService struct {
	recording bool
}

func NewTranscriptionService() *TranscriptionService {
	return &TranscriptionService{}
}

// StartRecording begins capturing audio from the microphone.
func (s *TranscriptionService) StartRecording() {
	s.recording = true
}

// StopRecording stops capturing and returns the transcribed text.
func (s *TranscriptionService) StopRecording() string {
	s.recording = false
	return "" // TODO: implement whisper transcription
}

// IsRecording returns the current recording state.
func (s *TranscriptionService) IsRecording() bool {
	return s.recording
}
