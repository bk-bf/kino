// Package config handles loading and writing the YAML config file.
package config

import (
	"log/slog"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"github.com/bk-bf/kino/internal/jellyfin"
)

// PreviewConfig holds image-preview related settings.
type PreviewConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	WidthPercent int    `mapstructure:"width_percent"`
	Protocol     string `mapstructure:"protocol"`
	DebounceMs   int    `mapstructure:"debounce_ms"`
	ShowBackdrop bool   `mapstructure:"show_backdrop"`
}

// AppConfig is the fully parsed configuration.
type AppConfig struct {
	Preview PreviewConfig
}

// DefaultPreviewConfig returns sensible defaults.
func DefaultPreviewConfig() PreviewConfig {
	return PreviewConfig{
		Enabled:      true,
		WidthPercent: 40,
		Protocol:     "auto",
		DebounceMs:   80,
		ShowBackdrop: false,
	}
}

// ReadPreviewConfig reads the preview sub-section from viper.
func ReadPreviewConfig() PreviewConfig {
	cfg := DefaultPreviewConfig()
	if viper.IsSet("preview.enabled") {
		cfg.Enabled = viper.GetBool("preview.enabled")
	}
	if viper.IsSet("preview.width_percent") {
		cfg.WidthPercent = viper.GetInt("preview.width_percent")
	}
	if viper.IsSet("preview.protocol") {
		cfg.Protocol = viper.GetString("preview.protocol")
	}
	if viper.IsSet("preview.debounce_ms") {
		cfg.DebounceMs = viper.GetInt("preview.debounce_ms")
	}
	if viper.IsSet("preview.show_backdrop") {
		cfg.ShowBackdrop = viper.GetBool("preview.show_backdrop")
	}
	return cfg
}

// Run initialises viper from the config file at path, prompts for missing
// credentials via a small Bubbletea form, and returns a ready *jellyfin.Client.
// Returns nil if the user quit the setup form.
func Run(clientVersion, path string) (*jellyfin.Client, PreviewConfig) {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		panic(err)
	}

	configModified := false

	if err := viper.ReadInConfig(); err != nil {
		configModified = true
	}

	host := viper.GetString("host")
	username := viper.GetString("username")
	password := viper.GetString("password")

	device := viper.GetString("device")
	if device == "" {
		device, _ = os.Hostname()
		viper.Set("device", device)
		configModified = true
	}
	deviceID := viper.GetString("device_id")
	if deviceID == "" {
		deviceID = uuid.NewString()
		viper.Set("device_id", deviceID)
		configModified = true
	}
	if viper.GetString("client_version") != clientVersion {
		viper.Set("client_version", clientVersion)
		configModified = true
	}
	token := viper.GetString("token")
	userID := viper.GetString("user_id")

	// set preview defaults so they appear in the written config
	if !viper.IsSet("preview.enabled") {
		viper.SetDefault("preview.enabled", true)
	}
	if !viper.IsSet("preview.width_percent") {
		viper.SetDefault("preview.width_percent", 40)
	}
	if !viper.IsSet("preview.protocol") {
		viper.SetDefault("preview.protocol", "auto")
	}
	if !viper.IsSet("preview.debounce_ms") {
		viper.SetDefault("preview.debounce_ms", 80)
	}
	if !viper.IsSet("preview.show_backdrop") {
		viper.SetDefault("preview.show_backdrop", false)
	}

	if configModified {
		if err := viper.WriteConfig(); err != nil {
			if err := viper.SafeWriteConfig(); err != nil {
				panic(err)
			}
		}
	}

	previewCfg := ReadPreviewConfig()

	if host != "" && username != "" {
		client, err := jellyfin.NewClient(host, username, password, device, deviceID, clientVersion, token, userID)
		if err == nil {
			if client.Token != token || client.UserID != userID {
				viper.Set("token", client.Token)
				viper.Set("user_id", client.UserID)
				slog.Info("updating token and user id")
				if err := viper.WriteConfig(); err != nil {
					_ = viper.SafeWriteConfig()
				}
			}
			return client, previewCfg
		}
		slog.Error("failed to create client", "err", err)
	}

	// Fall back to interactive setup form
	m, err := tea.NewProgram(initialSetupModel(), tea.WithAltScreen()).Run()
	if err != nil {
		panic(err)
	}
	sm := m.(setupModel)
	if sm.client == nil {
		return nil, previewCfg
	}
	return sm.client, previewCfg
}
