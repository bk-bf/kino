package main

import (
	"io"
	"log"
	"log/slog"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/adrg/xdg"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/pflag"

	"github.com/bk-bf/kino/config"
	internalcfg "github.com/bk-bf/kino/internal/config"
	"github.com/bk-bf/kino/internal/jellyfinSvc"
	"github.com/bk-bf/kino/internal/library"
	"github.com/bk-bf/kino/internal/player"
	"github.com/bk-bf/kino/internal/playlist"
	"github.com/bk-bf/kino/internal/preview"
	"github.com/bk-bf/kino/internal/search"
	"github.com/bk-bf/kino/internal/store"
	"github.com/bk-bf/kino/internal/tui"
)

var (
	version = "unknown"
	commit  = ""
	date    = ""
)

func main() {
	if version == "unknown" {
		if info, ok := debug.ReadBuildInfo(); ok {
			version = info.Main.Version
		}
	}

	cfgPath := pflag.StringP("config", "c", filepath.Join(xdg.ConfigHome, "jellyfin-tui", "config.yaml"), "config file path")
	debugPath := pflag.StringP("debug", "d", "", "debug log file path (enables debug logging)")
	printVersion := pflag.BoolP("version", "v", false, "show version")
	help := pflag.BoolP("help", "h", false, "show help")
	pflag.Parse()

	if *help {
		println("Usage:  jellyfin-tui [OPTIONS]")
		println()
		println("Options:")
		pflag.PrintDefaults()
		return
	}

	if *printVersion {
		println("version", version)
		if commit != "" {
			println("commit", commit)
		}
		if date != "" {
			println("date", date)
		}
		return
	}

	if *debugPath != "" {
		f, err := tea.LogToFile(*debugPath, "")
		if err != nil {
			panic(err)
		}
		defer f.Close()
		slog.SetDefault(
			slog.New(
				slog.NewJSONHandler(log.Writer(), &slog.HandlerOptions{
					Level: slog.LevelDebug,
					ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
						if a.Key == slog.TimeKey {
							return slog.String(a.Key, a.Value.Time().Format(time.DateTime))
						}
						return a
					},
				}),
			),
		)
		slog.Info("enabled debug logging")
	} else {
		log.SetOutput(io.Discard)
	}

	jfClient, previewCfg, subtitleCfg := config.Run(version, *cfgPath)
	if jfClient == nil {
		return
	}

	// Wire up services
	svcClient := jellyfinSvc.New(jfClient)
	memStore := store.New()

	libSvc := library.NewService(svcClient, memStore, nil)
	playlistSvc := playlist.NewService(svcClient, memStore, nil)
	searchSvc := search.NewService(memStore)

	launcher := player.NewLauncher("mpv", subtitleCfg.SubtitleMpvArgs(), "", nil)
	playerSvc := player.NewService(launcher, svcClient, nil)

	uiConfig := internalcfg.DefaultUIConfig()

	// Set up image preview
	var previewModel *preview.Model
	if previewCfg.Enabled {
		proto := preview.DetectProtocol(previewCfg.Protocol)
		pm := preview.New(jfClient.Host, proto)
		previewModel = &pm
	}

	m := tui.NewModel(memStore, libSvc, playlistSvc, searchSvc, playerSvc, uiConfig, previewModel)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
