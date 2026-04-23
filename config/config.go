// Package config handles loading and writing the YAML config file.
package config

import (
	"fmt"
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

// SubtitleConfig holds subtitle appearance settings passed to mpv.
type SubtitleConfig struct {
	// Font family name, e.g. "Liberation Sans"
	Font string `mapstructure:"font"`
	// Font size in points (mpv default: 55)
	FontSize int `mapstructure:"font_size"`
	// Primary colour in RRGGBB hex (e.g. "FFFFFF" for white)
	Color string `mapstructure:"color"`
	// Border (outline) size in pixels (mpv default: 3)
	BorderSize float64 `mapstructure:"border_size"`
	// Shadow offset in pixels (mpv default: 0)
	ShadowOffset float64 `mapstructure:"shadow_offset"`
	// Bold text
	Bold bool `mapstructure:"bold"`
}

// AppConfig is the fully parsed configuration.
type AppConfig struct {
	Preview   PreviewConfig
	Subtitles SubtitleConfig
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

// DefaultSubtitleConfig returns mpv's built-in defaults (no overrides).
func DefaultSubtitleConfig() SubtitleConfig {
	return SubtitleConfig{
		Font:         "",
		FontSize:     0,
		Color:        "",
		BorderSize:   -1,
		ShadowOffset: -1,
		Bold:         false,
	}
}

// ReadSubtitleConfig reads the subtitles sub-section from viper.
func ReadSubtitleConfig() SubtitleConfig {
	cfg := DefaultSubtitleConfig()
	if viper.IsSet("subtitles.font") {
		cfg.Font = viper.GetString("subtitles.font")
	}
	if viper.IsSet("subtitles.font_size") {
		cfg.FontSize = viper.GetInt("subtitles.font_size")
	}
	if viper.IsSet("subtitles.color") {
		cfg.Color = viper.GetString("subtitles.color")
	}
	if viper.IsSet("subtitles.border_size") {
		cfg.BorderSize = viper.GetFloat64("subtitles.border_size")
	}
	if viper.IsSet("subtitles.shadow_offset") {
		cfg.ShadowOffset = viper.GetFloat64("subtitles.shadow_offset")
	}
	if viper.IsSet("subtitles.bold") {
		cfg.Bold = viper.GetBool("subtitles.bold")
	}
	return cfg
}

// SubtitleMpvArgs converts SubtitleConfig into mpv command-line flags.
func (s SubtitleConfig) SubtitleMpvArgs() []string {
	var args []string
	if s.Font != "" {
		args = append(args, "--sub-font="+s.Font)
	}
	if s.FontSize > 0 {
		args = append(args, fmt.Sprintf("--sub-font-size=%d", s.FontSize))
	}
	if s.Color != "" {
		args = append(args, "--sub-color=#"+s.Color)
	}
	if s.BorderSize >= 0 {
		args = append(args, fmt.Sprintf("--sub-border-size=%.1f", s.BorderSize))
	}
	if s.ShadowOffset >= 0 {
		args = append(args, fmt.Sprintf("--sub-shadow-offset=%.1f", s.ShadowOffset))
	}
	if s.Bold {
		args = append(args, "--sub-bold=yes")
	}
	return args
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
func Run(clientVersion, path string) (*jellyfin.Client, PreviewConfig, SubtitleConfig) {
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
	subtitleCfg := ReadSubtitleConfig()

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
			return client, previewCfg, subtitleCfg
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
		return nil, previewCfg, subtitleCfg
	}
	return sm.client, previewCfg, subtitleCfg
}
