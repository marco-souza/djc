package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Default configuration values
const (
	DefaultAudioFormat  = "flac"
	DefaultAudioQuality = "0"
	AppName             = "djc"
)

// Config holds all application configuration
type Config struct {
	// Download settings
	DownloadDir  string `yaml:"download_dir"`
	AudioFormat  string `yaml:"audio_format"`
	AudioQuality string `yaml:"audio_quality"`

	// Database settings
	DatabasePath string `yaml:"database_path"`

	// Library settings
	LibraryDir string `yaml:"library_dir"`

	// Output template for downloads (yt-dlp format)
	OutputTemplate string `yaml:"output_template"`
}

// DefaultConfig returns a Config with default values
func DefaultConfig() *Config {
	dataDir := defaultDataDir()

	return &Config{
		DownloadDir:    filepath.Join(dataDir, "downloads"),
		AudioFormat:    DefaultAudioFormat,
		AudioQuality:   DefaultAudioQuality,
		DatabasePath:   filepath.Join(dataDir, "library.db"),
		LibraryDir:     dataDir,
		OutputTemplate: "%(playlist)s/%(title)s.%(ext)s",
	}
}

// Load loads configuration from file or creates default
func Load() (*Config, error) {
	configPath, err := ConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %w", err)
	}

	// If config doesn't exist, create default
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg := DefaultConfig()
		if err := cfg.Save(); err != nil {
			return nil, fmt.Errorf("failed to save default config: %w", err)
		}
		return cfg, nil
	}

	// Read existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults for missing fields
	cfg.applyDefaults()

	return cfg, nil
}

// Save writes configuration to file
func (c *Config) Save() error {
	configPath, err := ConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	// Ensure directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// applyDefaults fills in missing values with defaults
func (c *Config) applyDefaults() {
	defaults := DefaultConfig()

	if c.DownloadDir == "" {
		c.DownloadDir = defaults.DownloadDir
	}
	if c.AudioFormat == "" {
		c.AudioFormat = defaults.AudioFormat
	}
	if c.AudioQuality == "" {
		c.AudioQuality = defaults.AudioQuality
	}
	if c.DatabasePath == "" {
		c.DatabasePath = defaults.DatabasePath
	}
	if c.LibraryDir == "" {
		c.LibraryDir = defaults.LibraryDir
	}
	if c.OutputTemplate == "" {
		c.OutputTemplate = defaults.OutputTemplate
	}
}

// ConfigPath returns the path to the config file
// Uses XDG Base Directory specification:
// - Linux: $XDG_CONFIG_HOME/djc/config.yaml (defaults to ~/.config/djc/config.yaml)
// - macOS: ~/Library/Application Support/djc/config.yaml (via os.UserConfigDir)
func ConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, AppName, "config.yaml"), nil
}

// DataDir returns the path to the data directory
// Uses XDG Base Directory specification:
// - Linux: $XDG_DATA_HOME/djc (defaults to ~/.local/share/djc)
// - macOS: ~/Library/Application Support/djc
func DataDir() (string, error) {
	// Check for XDG_DATA_HOME first (Linux)
	if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
		return filepath.Join(dataHome, AppName), nil
	}

	// Fall back to UserConfigDir (works on both macOS and Linux)
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, AppName), nil
}

// defaultDataDir returns the default data directory
func defaultDataDir() string {
	dir, err := DataDir()
	if err != nil {
		// Fallback to home directory
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "."+AppName)
	}
	return dir
}
