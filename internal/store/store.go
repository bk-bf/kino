// Package store provides an in-memory implementation of domain.Store.
package store

import (
	"sync"

	"github.com/bk-bf/kino/internal/domain"
)

// MemStore is a simple thread-safe in-memory cache.
type MemStore struct {
	mu sync.RWMutex

	libraries     []domain.Library
	librariesOK   bool
	movies        map[string][]*domain.MediaItem // libID → items
	moviesTS      map[string]int64               // libID → serverTS
	shows         map[string][]*domain.Show
	showsTS       map[string]int64
	mixed         map[string][]domain.ListItem
	mixedTS       map[string]int64
	seasons       map[string][]*domain.Season    // libID+"/"+showID → items
	episodes      map[string][]*domain.MediaItem // libID+"/"+showID+"/"+seasonID → items
	playlists     []*domain.Playlist
	playlistsOK   bool
	playlistItems map[string][]*domain.MediaItem // playlistID → items
}

// New creates a new MemStore.
func New() *MemStore {
	return &MemStore{
		movies:        make(map[string][]*domain.MediaItem),
		moviesTS:      make(map[string]int64),
		shows:         make(map[string][]*domain.Show),
		showsTS:       make(map[string]int64),
		mixed:         make(map[string][]domain.ListItem),
		mixedTS:       make(map[string]int64),
		seasons:       make(map[string][]*domain.Season),
		episodes:      make(map[string][]*domain.MediaItem),
		playlistItems: make(map[string][]*domain.MediaItem),
	}
}

func (s *MemStore) GetLibraries() ([]domain.Library, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.libraries, s.librariesOK
}

func (s *MemStore) SaveLibraries(libs []domain.Library) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.libraries = libs
	s.librariesOK = true
	return nil
}

func (s *MemStore) GetMovies(libID string) ([]*domain.MediaItem, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.movies[libID]
	return v, ok
}

func (s *MemStore) SaveMovies(libID string, movies []*domain.MediaItem, serverTS int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.movies[libID] = movies
	s.moviesTS[libID] = serverTS
	return nil
}

func (s *MemStore) GetShows(libID string) ([]*domain.Show, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.shows[libID]
	return v, ok
}

func (s *MemStore) SaveShows(libID string, shows []*domain.Show, serverTS int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shows[libID] = shows
	s.showsTS[libID] = serverTS
	return nil
}

func (s *MemStore) GetMixedContent(libID string) ([]domain.ListItem, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.mixed[libID]
	return v, ok
}

func (s *MemStore) SaveMixedContent(libID string, items []domain.ListItem, serverTS int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mixed[libID] = items
	s.mixedTS[libID] = serverTS
	return nil
}

func (s *MemStore) GetSeasons(libID, showID string) ([]*domain.Season, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.seasons[libID+"/"+showID]
	return v, ok
}

func (s *MemStore) SaveSeasons(libID, showID string, seasons []*domain.Season) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seasons[libID+"/"+showID] = seasons
	return nil
}

func (s *MemStore) GetEpisodes(libID, showID, seasonID string) ([]*domain.MediaItem, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.episodes[libID+"/"+showID+"/"+seasonID]
	return v, ok
}

func (s *MemStore) SaveEpisodes(libID, showID, seasonID string, episodes []*domain.MediaItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.episodes[libID+"/"+showID+"/"+seasonID] = episodes
	return nil
}

func (s *MemStore) GetPlaylists() ([]*domain.Playlist, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.playlists, s.playlistsOK
}

func (s *MemStore) SavePlaylists(playlists []*domain.Playlist) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.playlists = playlists
	s.playlistsOK = true
	return nil
}

func (s *MemStore) GetPlaylistItems(playlistID string) ([]*domain.MediaItem, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.playlistItems[playlistID]
	return v, ok
}

func (s *MemStore) SavePlaylistItems(playlistID string, items []*domain.MediaItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.playlistItems[playlistID] = items
	return nil
}

// IsValid always returns false (in-memory store never checks server timestamps).
func (s *MemStore) IsValid(libID string, serverTS int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if serverTS == 0 {
		// No server timestamp available; treat as always stale
		return false
	}
	if ts, ok := s.moviesTS[libID]; ok && ts == serverTS {
		return true
	}
	if ts, ok := s.showsTS[libID]; ok && ts == serverTS {
		return true
	}
	if ts, ok := s.mixedTS[libID]; ok && ts == serverTS {
		return true
	}
	return false
}

func (s *MemStore) InvalidateLibrary(libID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.movies, libID)
	delete(s.moviesTS, libID)
	delete(s.shows, libID)
	delete(s.showsTS, libID)
	delete(s.mixed, libID)
	delete(s.mixedTS, libID)
}

func (s *MemStore) InvalidateShow(libID, showID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.seasons, libID+"/"+showID)
	// Also invalidate all episodes for this show
	prefix := libID + "/" + showID + "/"
	for k := range s.episodes {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(s.episodes, k)
		}
	}
}

func (s *MemStore) InvalidateSeason(libID, showID, seasonID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.episodes, libID+"/"+showID+"/"+seasonID)
}

func (s *MemStore) InvalidatePlaylists() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.playlists = nil
	s.playlistsOK = false
}

func (s *MemStore) InvalidatePlaylistItems(playlistID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.playlistItems, playlistID)
}

func (s *MemStore) InvalidateAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.libraries = nil
	s.librariesOK = false
	s.movies = make(map[string][]*domain.MediaItem)
	s.moviesTS = make(map[string]int64)
	s.shows = make(map[string][]*domain.Show)
	s.showsTS = make(map[string]int64)
	s.mixed = make(map[string][]domain.ListItem)
	s.mixedTS = make(map[string]int64)
	s.seasons = make(map[string][]*domain.Season)
	s.episodes = make(map[string][]*domain.MediaItem)
	s.playlists = nil
	s.playlistsOK = false
	s.playlistItems = make(map[string][]*domain.MediaItem)
}

func (s *MemStore) Close() error { return nil }
