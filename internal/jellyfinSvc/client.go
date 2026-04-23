// Package jellyfinSvc provides adapters that implement domain interfaces
// backed by the Jellyfin API.
package jellyfinSvc

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bk-bf/kino/internal/domain"
	"github.com/bk-bf/kino/internal/jellyfin"
)

// Client wraps jellyfin.Client and implements domain.LibraryClient,
// domain.PlaybackClient, and domain.PlaylistClient.
type Client struct {
	jf *jellyfin.Client
}

// New creates a new Client.
func New(jf *jellyfin.Client) *Client {
	return &Client{jf: jf}
}

// --------------------------------------------------------------------------
// domain.LibraryClient
// --------------------------------------------------------------------------

func (c *Client) GetLibraries(ctx context.Context) ([]domain.Library, error) {
	return c.jf.GetLibraries(ctx)
}

func (c *Client) GetMovies(ctx context.Context, libID string, offset, limit int) ([]*domain.MediaItem, int, error) {
	return c.jf.GetMovies(ctx, libID, offset, limit)
}

func (c *Client) GetShows(ctx context.Context, libID string, offset, limit int) ([]*domain.Show, int, error) {
	return c.jf.GetShows(ctx, libID, offset, limit)
}

func (c *Client) GetMixedContent(ctx context.Context, libID string, offset, limit int) ([]domain.ListItem, int, error) {
	return c.jf.GetMixedContent(ctx, libID, offset, limit)
}

func (c *Client) GetSeasons(ctx context.Context, showID string) ([]*domain.Season, error) {
	return c.jf.GetDomainSeasons(ctx, showID)
}

func (c *Client) GetEpisodes(ctx context.Context, seasonID string) ([]*domain.MediaItem, error) {
	return c.jf.GetDomainEpisodes(ctx, seasonID)
}

func (c *Client) GetShowEpisodeCount(ctx context.Context, showID string) (int, error) {
	return c.jf.GetShowEpisodeCount(ctx, showID)
}

func (c *Client) GetSeasonCountsByLibrary(ctx context.Context, libID string) (map[string]int, error) {
	return c.jf.GetSeasonCountsByLibrary(ctx, libID)
}

// --------------------------------------------------------------------------
// domain.PlaybackClient
// --------------------------------------------------------------------------

func (c *Client) ResolvePlayableURL(ctx context.Context, itemID string) (string, error) {
	url := fmt.Sprintf("%s/Videos/%s/stream?static=true&api_key=%s",
		c.jf.Host, itemID, c.jf.Token)
	// Wrap as EDL so MPV handles it correctly (same as jfsh pattern)
	return fmt.Sprintf("edl://%%%d%%%s", len(url), url), nil
}

func (c *Client) GetSubtitleURLs(ctx context.Context, itemID string) ([]string, error) {
	// Use PlaybackInfo API — it returns properly resolved DeliveryUrls for playback.
	info, err := c.jf.GetPlaybackInfo(ctx, itemID)
	if err != nil {
		slog.Error("GetPlaybackInfo failed, falling back to GetItems", "itemID", itemID, "error", err)
		item, err2 := c.jf.GetItemWithMediaStreams(ctx, itemID)
		if err2 != nil {
			return nil, err2
		}
		subtitles := jellyfin.GetExternalSubtitleStreams(item)
		return c.buildSubtitleURLs(subtitles), nil
	}

	subtitles := jellyfin.GetExternalSubtitleStreamsFromPlaybackInfo(info, itemID)
	return c.buildSubtitleURLs(subtitles), nil
}

func (c *Client) buildSubtitleURLs(subtitles []jellyfin.ExternalSubtitleStream) []string {
	urls := make([]string, 0, len(subtitles))
	for _, sub := range subtitles {
		url := sub.URL
		if !strings.HasPrefix(url, "http") {
			url = c.jf.Host + url
		}
		if !strings.Contains(url, "api_key=") && !strings.Contains(url, "ApiKey=") {
			sep := "?"
			if strings.Contains(url, "?") {
				sep = "&"
			}
			url = fmt.Sprintf("%s%sapi_key=%s", url, sep, c.jf.Token)
		}
		slog.Info("subtitle URL", "title", sub.Title, "language", sub.Language, "url", url)
		urls = append(urls, url)
	}
	return urls
}

func (c *Client) MarkPlayed(ctx context.Context, itemID string) error {
	return c.jf.MarkPlayedByID(ctx, itemID)
}

func (c *Client) MarkUnplayed(ctx context.Context, itemID string) error {
	return c.jf.MarkUnplayedByID(ctx, itemID)
}

// --------------------------------------------------------------------------
// domain.PlaylistClient
// --------------------------------------------------------------------------

func (c *Client) GetPlaylists(ctx context.Context) ([]*domain.Playlist, error) {
	return c.jf.GetPlaylists(ctx)
}

func (c *Client) GetPlaylistItems(ctx context.Context, playlistID string) ([]*domain.MediaItem, error) {
	return c.jf.GetPlaylistItems(ctx, playlistID)
}

func (c *Client) CreatePlaylist(ctx context.Context, title string, itemIDs []string) (*domain.Playlist, error) {
	return c.jf.CreatePlaylist(ctx, title, itemIDs)
}

func (c *Client) AddToPlaylist(ctx context.Context, playlistID string, itemIDs []string) error {
	return c.jf.AddToPlaylist(ctx, playlistID, itemIDs)
}

func (c *Client) RemoveFromPlaylist(ctx context.Context, playlistID string, itemID string) error {
	return c.jf.RemoveFromPlaylist(ctx, playlistID, itemID)
}

func (c *Client) DeletePlaylist(ctx context.Context, playlistID string) error {
	return c.jf.DeletePlaylist(ctx, playlistID)
}
