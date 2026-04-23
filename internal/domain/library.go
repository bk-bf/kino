package domain

import "context"

// LibraryClient provides network operations for library browsing.
type LibraryClient interface {
	GetLibraries(ctx context.Context) ([]Library, error)
	GetMovies(ctx context.Context, libID string, offset, limit int) ([]*MediaItem, int, error)
	GetShows(ctx context.Context, libID string, offset, limit int) ([]*Show, int, error)
	GetMixedContent(ctx context.Context, libID string, offset, limit int) ([]ListItem, int, error)
	GetSeasons(ctx context.Context, showID string) ([]*Season, error)
	GetEpisodes(ctx context.Context, seasonID string) ([]*MediaItem, error)
	GetShowEpisodeCount(ctx context.Context, showID string) (int, error)
	// GetSeasonCountsByLibrary returns a map of seriesID → season count for all
	// series in the given library in a single API call.
	GetSeasonCountsByLibrary(ctx context.Context, libID string) (map[string]int, error)
}
