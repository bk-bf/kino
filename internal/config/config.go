// Package config holds application configuration types.
package config

// UIConfig holds UI configuration
type UIConfig struct {
	ShowWatchStatus   bool // Show watched/unwatched/in-progress indicators
	ShowLibraryCounts bool // Keep library item counts visible after sync
}

// DefaultUIConfig returns sensible defaults.
func DefaultUIConfig() UIConfig {
	return UIConfig{
		ShowWatchStatus:   true,
		ShowLibraryCounts: true,
	}
}
