# kino

A terminal UI for [Jellyfin](https://jellyfin.org/) media servers, written in Go.

Browse your movies, TV shows, and playlists, play media via mpv, and manage your watch history — all from the terminal.

## Features

- Miller-column navigation (parent / active / inspector)
- Browse Movies, TV Shows, and mixed libraries
- Episode drill-down for series (seasons → episodes)
- Inspector panel with metadata, episode count, and progress
- Sort by title, date added, release date, or community rating
- Filter / search within columns; global search across all libraries
- Playlist management (create, add to, delete)
- Mark items watched / unwatched
- Plays media via mpv
- Persists credentials and config via XDG paths

## Requirements

- [mpv](https://mpv.io/) — media playback
- A running Jellyfin server

## Installation

```
go install github.com/bk-bf/kino@latest
```

Or build from source:

```
git clone https://github.com/bk-bf/kino
cd kino
go build -o kino .
```

## Usage

```
kino
```

On first launch you will be prompted for your Jellyfin server URL, username, and password. Credentials are stored in your XDG config directory.

## Keybindings

| Key | Action |
|-----|--------|
| `j` / `k` | Move down / up |
| `h` / `l` or `←` / `→` | Navigate columns / drill in |
| `Enter` | Play selected item |
| `p` | Toggle play/pause |
| `i` | Toggle inspector panel |
| `s` | Sort modal |
| `/` | Filter current column |
| `g` | Global search |
| `P` | Playlist modal |
| `n` | New playlist |
| `w` | Mark watched |
| `u` | Mark unwatched |
| `r` | Refresh current column |
| `R` | Refresh all |
| `L` | Logout |
| `?` | Help |
| `q` | Quit |

## Image Preview

kino can display poster images in the inspector panel using terminal graphics protocols (Kitty, Sixel, or iTerm2). Set `preview.enabled: true` in the config file and optionally set `preview.protocol` (default: `auto`).

### tmux

The Kitty graphics protocol requires passthrough to be enabled in tmux (tmux ≥ 3.3). Add this to your tmux config:

```
set -g allow-passthrough on
```

Without this, images will silently not render when running inside tmux, even if the underlying terminal supports the Kitty protocol.

## Configuration

Config is stored at `$XDG_CONFIG_HOME/kino/` (typically `~/.config/kino/`).
