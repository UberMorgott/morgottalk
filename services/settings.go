package services

import "github.com/UberMorgott/transcribation/internal/config"

// SettingsService provides settings management to the frontend.
type SettingsService struct {
	cfg *config.Config
}

func NewSettingsService() *SettingsService {
	cfg, _ := config.Load()
	return &SettingsService{cfg: cfg}
}

// GetSettings returns the current configuration.
func (s *SettingsService) GetSettings() *config.Config {
	return s.cfg
}

// SaveSettings persists the configuration.
func (s *SettingsService) SaveSettings(cfg config.Config) error {
	s.cfg = &cfg
	return config.Save(&cfg)
}
