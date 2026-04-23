package jellyfin

import (
	"context"
	"fmt"
	"time"

	"github.com/bk-bf/kino/internal/domain"
	"github.com/sj14/jellyfin-go/api"
)

// GetLibraries returns the top-level media libraries for the current user.
func (c *Client) GetLibraries(ctx context.Context) ([]domain.Library, error) {
	res, _, err := c.api.UserViewsAPI.GetUserViews(ctx).Execute()
	if err != nil {
		return nil, err
	}
	var libs []domain.Library
	for _, item := range res.Items {
		libType := mapCollectionType(string(item.GetCollectionType()))
		libs = append(libs, domain.Library{
			ID:   item.GetId(),
			Name: item.GetName(),
			Type: libType,
		})
	}
	return libs, nil
}

// mapCollectionType maps Jellyfin CollectionType to our domain library type.
func mapCollectionType(ct string) string {
	switch ct {
	case "movies":
		return "movie"
	case "tvshows":
		return "show"
	default:
		return "mixed"
	}
}

// GetMovies fetches a page of movies from a library.
func (c *Client) GetMovies(ctx context.Context, libID string, offset, limit int) ([]*domain.MediaItem, int, error) {
	req := c.api.ItemsAPI.GetItems(ctx).
		UserId(c.UserID).
		ParentId(libID).
		IncludeItemTypes([]api.BaseItemKind{api.BASEITEMKIND_MOVIE}).
		Recursive(true).
		Fields([]api.ItemFields{
			api.ITEMFIELDS_MEDIA_STREAMS,
			api.ITEMFIELDS_OVERVIEW,
			api.ITEMFIELDS_SORT_NAME,
			api.ITEMFIELDS_DATE_CREATED,
		}).
		SortBy([]api.ItemSortBy{api.ITEMSORTBY_SORT_NAME}).
		SortOrder([]api.SortOrder{api.SORTORDER_ASCENDING}).
		StartIndex(int32(offset)).
		Limit(int32(limit))

	res, _, err := req.Execute()
	if err != nil {
		return nil, 0, err
	}
	items := make([]*domain.MediaItem, 0, len(res.Items))
	for _, it := range res.Items {
		items = append(items, mapBaseItemToMediaItem(c.Host, it))
	}
	return items, int(res.GetTotalRecordCount()), nil
}

// GetShows fetches a page of TV series from a library.
func (c *Client) GetShows(ctx context.Context, libID string, offset, limit int) ([]*domain.Show, int, error) {
	req := c.api.ItemsAPI.GetItems(ctx).
		UserId(c.UserID).
		ParentId(libID).
		IncludeItemTypes([]api.BaseItemKind{api.BASEITEMKIND_SERIES}).
		Recursive(true).
		Fields([]api.ItemFields{
			api.ITEMFIELDS_OVERVIEW,
			api.ITEMFIELDS_SORT_NAME,
			api.ITEMFIELDS_DATE_CREATED,
			api.ITEMFIELDS_CHILD_COUNT,
		}).
		SortBy([]api.ItemSortBy{api.ITEMSORTBY_SORT_NAME}).
		SortOrder([]api.SortOrder{api.SORTORDER_ASCENDING}).
		StartIndex(int32(offset)).
		Limit(int32(limit))

	res, _, err := req.Execute()
	if err != nil {
		return nil, 0, err
	}
	shows := make([]*domain.Show, 0, len(res.Items))
	for _, it := range res.Items {
		shows = append(shows, mapBaseItemToShow(c.Host, libID, it))
	}
	return shows, int(res.GetTotalRecordCount()), nil
}

// GetMixedContent fetches a page of movies and series from a library.
func (c *Client) GetMixedContent(ctx context.Context, libID string, offset, limit int) ([]domain.ListItem, int, error) {
	req := c.api.ItemsAPI.GetItems(ctx).
		UserId(c.UserID).
		ParentId(libID).
		IncludeItemTypes([]api.BaseItemKind{api.BASEITEMKIND_MOVIE, api.BASEITEMKIND_SERIES}).
		Recursive(true).
		Fields([]api.ItemFields{
			api.ITEMFIELDS_MEDIA_STREAMS,
			api.ITEMFIELDS_OVERVIEW,
			api.ITEMFIELDS_SORT_NAME,
			api.ITEMFIELDS_DATE_CREATED,
			api.ITEMFIELDS_CHILD_COUNT,
		}).
		SortBy([]api.ItemSortBy{api.ITEMSORTBY_SORT_NAME}).
		SortOrder([]api.SortOrder{api.SORTORDER_ASCENDING}).
		StartIndex(int32(offset)).
		Limit(int32(limit))

	res, _, err := req.Execute()
	if err != nil {
		return nil, 0, err
	}
	items := make([]domain.ListItem, 0, len(res.Items))
	for _, it := range res.Items {
		switch it.GetType() {
		case api.BASEITEMKIND_SERIES:
			items = append(items, mapBaseItemToShow(c.Host, libID, it))
		default:
			items = append(items, mapBaseItemToMediaItem(c.Host, it))
		}
	}
	return items, int(res.GetTotalRecordCount()), nil
}

// GetDomainSeasons returns all seasons for a show.
func (c *Client) GetDomainSeasons(ctx context.Context, showID string) ([]*domain.Season, error) {
	res, _, err := c.api.TvShowsAPI.GetSeasons(ctx, showID).
		UserId(c.UserID).
		Execute()
	if err != nil {
		return nil, err
	}
	showTitle := ""
	seasons := make([]*domain.Season, 0, len(res.Items))
	for _, it := range res.Items {
		if showTitle == "" {
			showTitle = it.GetSeriesName()
		}
		s := &domain.Season{
			ID:           it.GetId(),
			ShowID:       showID,
			ShowTitle:    it.GetSeriesName(),
			SeasonNum:    int(it.GetIndexNumber()),
			Title:        it.GetName(),
			EpisodeCount: int(it.GetChildCount()),
			ThumbURL:     fmt.Sprintf("%s/Items/%s/Images/Primary", c.Host, it.GetId()),
		}
		if ud, ok := it.GetUserDataOk(); ok {
			s.UnwatchedCount = int(ud.GetUnplayedItemCount())
		}
		seasons = append(seasons, s)
	}
	_ = showTitle
	return seasons, nil
}

// GetDomainEpisodes returns all episodes for a season.
func (c *Client) GetDomainEpisodes(ctx context.Context, seasonID string) ([]*domain.MediaItem, error) {
	res, _, err := c.api.ItemsAPI.GetItems(ctx).
		UserId(c.UserID).
		ParentId(seasonID).
		IncludeItemTypes([]api.BaseItemKind{api.BASEITEMKIND_EPISODE}).
		Fields([]api.ItemFields{
			api.ITEMFIELDS_MEDIA_STREAMS,
			api.ITEMFIELDS_OVERVIEW,
		}).
		SortBy([]api.ItemSortBy{api.ITEMSORTBY_SORT_NAME}).
		SortOrder([]api.SortOrder{api.SORTORDER_ASCENDING}).
		Execute()
	if err != nil {
		return nil, err
	}
	items := make([]*domain.MediaItem, 0, len(res.Items))
	for _, it := range res.Items {
		mi := mapBaseItemToMediaItem(c.Host, it)
		mi.Type = domain.MediaTypeEpisode
		items = append(items, mi)
	}
	return items, nil
}

// MarkPlayedByID marks an item as played.
func (c *Client) MarkPlayedByID(ctx context.Context, itemID string) error {
	_, _, err := c.api.PlaystateAPI.MarkPlayedItem(ctx, itemID).Execute()
	return err
}

// MarkUnplayedByID marks an item as unplayed.
func (c *Client) MarkUnplayedByID(ctx context.Context, itemID string) error {
	_, _, err := c.api.PlaystateAPI.MarkUnplayedItem(ctx, itemID).Execute()
	return err
}

// GetPlaybackInfo fetches playback info for an item which includes properly
// resolved MediaSources with MediaStreams and DeliveryUrls.
func (c *Client) GetPlaybackInfo(ctx context.Context, itemID string) (*api.PlaybackInfoResponse, error) {
	res, _, err := c.api.MediaInfoAPI.GetPlaybackInfo(ctx, itemID).
		UserId(c.UserID).
		Execute()
	if err != nil {
		return nil, err
	}
	return res, nil
}

// GetItemWithMediaStreams fetches a single item including its media streams and sources.
func (c *Client) GetItemWithMediaStreams(ctx context.Context, itemID string) (Item, error) {
	res, _, err := c.api.ItemsAPI.GetItems(ctx).
		UserId(c.UserID).
		Ids([]string{itemID}).
		Fields([]api.ItemFields{
			api.ITEMFIELDS_MEDIA_STREAMS,
			api.ITEMFIELDS_MEDIA_SOURCES,
		}).
		Execute()
	if err != nil {
		return Item{}, err
	}
	if len(res.Items) == 0 {
		return Item{}, fmt.Errorf("item not found: %s", itemID)
	}
	return res.Items[0], nil
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func mapBaseItemToMediaItem(host string, it api.BaseItemDto) *domain.MediaItem {
	mi := &domain.MediaItem{
		ID:       it.GetId(),
		Title:    it.GetName(),
		Summary:  it.GetOverview(),
		Year:     int(it.GetProductionYear()),
		Rating:   float64(it.GetCommunityRating()),
		ThumbURL: fmt.Sprintf("%s/Items/%s/Images/Primary", host, it.GetId()),
		ArtURL:   fmt.Sprintf("%s/Items/%s/Images/Backdrop/0", host, it.GetId()),
		Type:     domain.MediaTypeMovie,
	}

	if st := it.GetSortName(); st != "" {
		mi.SortTitle = st
	}
	if cr := it.GetOfficialRating(); cr != "" {
		mi.ContentRating = cr
	}
	if ticks := it.GetRunTimeTicks(); ticks > 0 {
		mi.Duration = time.Duration(ticks) * 100 * time.Nanosecond
	}
	if ud, ok := it.GetUserDataOk(); ok {
		mi.IsPlayed = ud.GetPlayed()
		if pos := ud.GetPlaybackPositionTicks(); pos > 0 {
			mi.ViewOffset = time.Duration(pos) * 100 * time.Nanosecond
		}
	}
	if dc, ok := it.GetDateCreatedOk(); ok && dc != nil && !dc.IsZero() {
		mi.AddedAt = dc.Unix()
	}

	switch it.GetType() {
	case api.BASEITEMKIND_EPISODE:
		mi.Type = domain.MediaTypeEpisode
		mi.ShowTitle = it.GetSeriesName()
		mi.ShowID = it.GetSeriesId()
		mi.SeasonNum = int(it.GetParentIndexNumber())
		mi.EpisodeNum = int(it.GetIndexNumber())
		mi.ParentID = it.GetSeasonId()
	}

	for _, stream := range it.GetMediaStreams() {
		switch stream.GetType() {
		case "Video":
			mi.Width = int(stream.GetWidth())
			mi.Height = int(stream.GetHeight())
			mi.VideoCodec = stream.GetCodec()
		case "Audio":
			if mi.AudioCodec == "" {
				mi.AudioCodec = stream.GetCodec()
				mi.AudioChannels = int(stream.GetChannels())
			}
		}
	}

	return mi
}

// --------------------------------------------------------------------------
// Playlist methods
// --------------------------------------------------------------------------

// GetPlaylists returns all playlists for the current user.
func (c *Client) GetPlaylists(ctx context.Context) ([]*domain.Playlist, error) {
	res, _, err := c.api.ItemsAPI.GetItems(ctx).
		UserId(c.UserID).
		IncludeItemTypes([]api.BaseItemKind{api.BASEITEMKIND_PLAYLIST}).
		Recursive(true).
		Execute()
	if err != nil {
		return nil, err
	}
	playlists := make([]*domain.Playlist, 0, len(res.Items))
	for _, it := range res.Items {
		p := &domain.Playlist{
			ID:    it.GetId(),
			Title: it.GetName(),
		}
		if it.GetChildCount() > 0 {
			p.ItemCount = int(it.GetChildCount())
		}
		playlists = append(playlists, p)
	}
	return playlists, nil
}

// GetPlaylistItems returns all items in a playlist.
func (c *Client) GetPlaylistItems(ctx context.Context, playlistID string) ([]*domain.MediaItem, error) {
	res, _, err := c.api.PlaylistsAPI.GetPlaylistItems(ctx, playlistID).
		UserId(c.UserID).
		Execute()
	if err != nil {
		return nil, err
	}
	items := make([]*domain.MediaItem, 0, len(res.Items))
	for _, it := range res.Items {
		items = append(items, mapBaseItemToMediaItem(c.Host, it))
	}
	return items, nil
}

// CreatePlaylist creates a new playlist.
func (c *Client) CreatePlaylist(ctx context.Context, title string, itemIDs []string) (*domain.Playlist, error) {
	dto := api.NewCreatePlaylistDto()
	dto.SetName(title)
	if len(itemIDs) > 0 {
		dto.Ids = itemIDs
	}
	dto.UserId = *api.NewNullableString(&c.UserID)
	res, _, err := c.api.PlaylistsAPI.CreatePlaylist(ctx).CreatePlaylistDto(*dto).Execute()
	if err != nil {
		return nil, err
	}
	return &domain.Playlist{
		ID:    res.GetId(),
		Title: title,
	}, nil
}

// AddToPlaylist adds items to a playlist.
func (c *Client) AddToPlaylist(ctx context.Context, playlistID string, itemIDs []string) error {
	_, err := c.api.PlaylistsAPI.AddItemToPlaylist(ctx, playlistID).
		Ids(itemIDs).
		UserId(c.UserID).
		Execute()
	return err
}

// RemoveFromPlaylist removes an item from a playlist.
func (c *Client) RemoveFromPlaylist(ctx context.Context, playlistID string, itemID string) error {
	_, err := c.api.PlaylistsAPI.RemoveItemFromPlaylist(ctx, playlistID).
		EntryIds([]string{itemID}).
		Execute()
	return err
}

// DeletePlaylist deletes a playlist by removing it from the library.
func (c *Client) DeletePlaylist(ctx context.Context, playlistID string) error {
	_, err := c.api.LibraryAPI.DeleteItem(ctx, playlistID).Execute()
	return err
}

// GetSeasonCountsByLibrary returns a map of seriesID → season count for all series
// in the given library, fetched in a single API call.
func (c *Client) GetSeasonCountsByLibrary(ctx context.Context, libID string) (map[string]int, error) {
	res, _, err := c.api.ItemsAPI.GetItems(ctx).
		UserId(c.UserID).
		ParentId(libID).
		IncludeItemTypes([]api.BaseItemKind{api.BASEITEMKIND_SEASON}).
		Recursive(true).
		Execute()
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int, len(res.Items))
	for _, it := range res.Items {
		sid := it.GetSeriesId()
		counts[sid]++
	}
	return counts, nil
}

// GetShowEpisodeCount returns the total number of episodes for a given series.
func (c *Client) GetShowEpisodeCount(ctx context.Context, showID string) (int, error) {
	res, _, err := c.api.TvShowsAPI.GetEpisodes(ctx, showID).
		UserId(c.UserID).
		Limit(0).
		Execute()
	if err != nil {
		return 0, err
	}
	return int(res.GetTotalRecordCount()), nil
}

func mapBaseItemToShow(host, libID string, it api.BaseItemDto) *domain.Show {
	show := &domain.Show{
		ID:          it.GetId(),
		Title:       it.GetName(),
		LibraryID:   libID,
		Summary:     it.GetOverview(),
		Year:        int(it.GetProductionYear()),
		Rating:      float64(it.GetCommunityRating()),
		SeasonCount: int(it.GetChildCount()),
		ThumbURL:    fmt.Sprintf("%s/Items/%s/Images/Primary", host, it.GetId()),
		ArtURL:      fmt.Sprintf("%s/Items/%s/Images/Backdrop/0", host, it.GetId()),
	}
	if st := it.GetSortName(); st != "" {
		show.SortTitle = st
	}
	if cr := it.GetOfficialRating(); cr != "" {
		show.ContentRating = cr
	}
	if ud, ok := it.GetUserDataOk(); ok {
		show.UnwatchedCount = int(ud.GetUnplayedItemCount())
	}
	if dc, ok := it.GetDateCreatedOk(); ok && dc != nil && !dc.IsZero() {
		show.AddedAt = dc.Unix()
	}
	return show
}
