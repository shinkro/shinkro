package mapping

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/santhosh-tekuri/jsonschema/v5"
	_ "github.com/santhosh-tekuri/jsonschema/v5/httploader"
	"gopkg.in/yaml.v3"

	"github.com/shinkro/shinkro/internal/domain"
	"github.com/shinkro/shinkro/pkg/sharedhttp"
)

type Service interface {
	NewMap(ctx context.Context) (*domain.AnimeMap, error)
	CheckForAnimeinMap(ctx context.Context, anime *domain.AnimeUpdate) (*domain.AnimeMapDetails, error)
	ValidateMap(ctx context.Context, yamlPath string, isTVDB bool) error
	Store(ctx context.Context, m *domain.MapSettings) error
	Get(ctx context.Context) (*domain.MapSettings, error)
}

type service struct {
	log  zerolog.Logger
	repo domain.MappingRepo

	mu        sync.RWMutex
	cachedMap *domain.AnimeMap
}

func NewService(log zerolog.Logger, repo domain.MappingRepo) Service {
	return &service{
		log:  log.With().Str("module", "mapping").Logger(),
		repo: repo,
	}
}

func (s *service) Store(ctx context.Context, m *domain.MapSettings) error {
	if err := s.repo.Store(ctx, m); err != nil {
		return err
	}

	// Invalidate cache on store
	s.mu.Lock()
	s.cachedMap = nil
	s.mu.Unlock()

	s.log.Debug().Msg("mapping settings updated, cache invalidated")

	return nil
}

func (s *service) Get(ctx context.Context) (*domain.MapSettings, error) {
	return s.repo.Get(ctx)
}

// NewMap bypasses cache and always loads fresh mappings
func (s *service) NewMap(ctx context.Context) (*domain.AnimeMap, error) {
	return s.loadMap(ctx)
}

// loadMap encapsulates loading logic from repository
func (s *service) loadMap(ctx context.Context) (*domain.AnimeMap, error) {
	settings, err := s.repo.Get(ctx)
	if err != nil {
		return nil, err
	}

	animeMap := &domain.AnimeMap{
		AnimeTVShows: &domain.AnimeTVShows{},
		AnimeMovies:  &domain.AnimeMovies{},
	}

	shouldLoadLocalTVDB, shouldLoadLocalTMDB := settings.ShouldLoadLocal()
	localTVDBExists, localTMDBExists := s.checkLocalMapsExist(settings)
	
	// Load local maps if they exist and should be loaded
	if (shouldLoadLocalTVDB && localTVDBExists) || (shouldLoadLocalTMDB && localTMDBExists) {
		if err := s.loadLocalMaps(settings, animeMap, shouldLoadLocalTVDB && localTVDBExists, shouldLoadLocalTMDB && localTMDBExists); err != nil {
			return nil, err
		}
	}

	// Load community maps for any that weren't loaded locally
	needsTVDB := !(shouldLoadLocalTVDB && localTVDBExists)
	needsTMDB := !(shouldLoadLocalTMDB && localTMDBExists)
	
	if needsTVDB || needsTMDB {
		if err := s.loadCommunityMaps(ctx, animeMap, needsTVDB, needsTMDB); err != nil {
			return nil, err
		}
	}

	return animeMap, nil
}

// getCachedMap returns cached map or loads it if absent
func (s *service) getCachedMap(ctx context.Context) (*domain.AnimeMap, error) {
	s.mu.RLock()
	if s.cachedMap != nil {
		m := s.cachedMap
		s.mu.RUnlock()
		s.log.Debug().Msg("cache hit")
		return m, nil
	}
	s.mu.RUnlock()

	s.log.Debug().Msg("cache miss, loading map")
	m, err := s.loadMap(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.cachedMap = m
	s.mu.Unlock()
	return m, nil
}

// reloadMap forces a refresh of the cache
func (s *service) reloadMap(ctx context.Context) (*domain.AnimeMap, error) {
	m, err := s.loadMap(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.cachedMap = m
	s.mu.Unlock()

	s.log.Debug().Msg("reloaded map")

	return m, nil
}

func (s *service) CheckForAnimeinMap(ctx context.Context, anime *domain.AnimeUpdate) (*domain.AnimeMapDetails, error) {
	animeMap, err := s.getCachedMap(ctx)
	if err != nil {
		return nil, err
	}

	if details, found := s.checkMap(animeMap, anime); found {
		return details, nil
	}

	animeMap, err = s.reloadMap(ctx)
	if err != nil {
		return nil, err
	}

	s.log.Debug().Msg("reloaded map")

	if details, found := s.checkMap(animeMap, anime); found {
		return details, nil
	}

	return nil, errors.New("anime not found in map")
}

// checkMap encapsulates the lookup logic
func (s *service) checkMap(m *domain.AnimeMap, anime *domain.AnimeUpdate) (*domain.AnimeMapDetails, bool) {
	switch anime.Plex.Metadata.Type {
	case domain.PlexMovie:
		if inMap, movie := m.AnimeMovies.CheckMap(anime.SourceId); inMap {
			s.log.Debug().Msg("found anime movie in map")
			return &domain.AnimeMapDetails{Malid: movie.MALID, Start: 0}, true
		}

	case domain.PlexEpisode:
		if inMap, ep := m.AnimeTVShows.CheckMap(anime.SourceId, anime.SeasonNum, anime.EpisodeNum); inMap {
			s.log.Debug().Msg("found anime episode in map")
			return &domain.AnimeMapDetails{
				Malid:           ep.Malid,
				Start:           ep.Start,
				UseMapping:      ep.UseMapping,
				MappingType:     ep.MappingType,
				ExplicitEpisodes: ep.ExplicitEpisodes,
				SkipMalEpisodes: ep.SkipMalEpisodes,
			}, true
		}
	}
	return nil, false
}

func (s *service) ValidateMap(ctx context.Context, yamlPath string, isTVDB bool) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	var schemaURL domain.MapValidationUrls
	if isTVDB {
		schemaURL = domain.TVDBValidationUrl
	} else {
		schemaURL = domain.TMDBValidationUrl
	}

	raw, err := os.ReadFile(yamlPath)
	if err != nil {
		return errors.Errorf("could not read YAML file %q: %v", yamlPath, err)
	}

	var doc interface{}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return errors.Errorf("YAML parse error in %q: %v", yamlPath, err)
	}

	jsonData := toJSONCompatible(doc)
	compiler := jsonschema.NewCompiler()
	sch, err := compiler.Compile(string(schemaURL))
	if err != nil {
		return errors.Errorf("schema compile error for %s: %v", schemaURL, err)
	}

	if err := sch.Validate(jsonData); err != nil {
		s.log.Error().Err(err).Msgf("schema validation for %v custom map failed", yamlPath)
		return errors.Errorf("schema validation failed: %v", err)
	}

	return nil
}

// loadCommunityMaps loads mapping data from remote URLs (GitHub)
func (s *service) loadCommunityMaps(ctx context.Context, animeMap *domain.AnimeMap, loadTVDB, loadTMDB bool) error {
	if loadTVDB {
		if err := s.loadYamlFromURL(ctx, string(domain.CommunityMapTVDB), animeMap.AnimeTVShows); err != nil {
			return errors.Wrap(err, "failed to load TVDB community map")
		}
	}

	if loadTMDB {
		if err := s.loadYamlFromURL(ctx, string(domain.CommunityMapTMDB), animeMap.AnimeMovies); err != nil {
			return errors.Wrap(err, "failed to load TMDB community map")
		}
	}

	return nil
}

// loadLocalMaps loads mapping data from local files
func (s *service) loadLocalMaps(settings *domain.MapSettings, animeMap *domain.AnimeMap, loadTVDB, loadTMDB bool) error {
	if loadTVDB {
		if err := s.loadYamlFromFile(settings.CustomMapTVDBPath, animeMap.AnimeTVShows); err != nil {
			return errors.Wrapf(err, "failed to load local TVDB map from %s", settings.CustomMapTVDBPath)
		}
	}

	if loadTMDB {
		if err := s.loadYamlFromFile(settings.CustomMapTMDBPath, animeMap.AnimeMovies); err != nil {
			return errors.Wrapf(err, "failed to load local TMDB map from %s", settings.CustomMapTMDBPath)
		}
	}

	return nil
}

// loadYamlFromURL fetches YAML data from a URL and unmarshals it
func (s *service) loadYamlFromURL(ctx context.Context, url string, target interface{}) error {
	client := &http.Client{Transport: sharedhttp.Transport}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", sharedhttp.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(body, target); err != nil {
		return err
	}

	return nil
}

// loadYamlFromFile reads YAML data from a file and unmarshals it
func (s *service) loadYamlFromFile(filePath string, target interface{}) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	body, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(body, target); err != nil {
		return err
	}

	return nil
}

// fileExists checks if a file exists
func (s *service) fileExists(path string) bool {
	_, err := os.Open(path)
	return err == nil
}

// checkLocalMapsExist checks which local map files actually exist
func (s *service) checkLocalMapsExist(settings *domain.MapSettings) (bool, bool) {
	tvdbExists := s.fileExists(settings.CustomMapTVDBPath)
	tmdbExists := s.fileExists(settings.CustomMapTMDBPath)
	return tvdbExists, tmdbExists
}

func toJSONCompatible(v interface{}) interface{} {
	switch x := v.(type) {
	case map[interface{}]interface{}:
		m2 := make(map[string]interface{}, len(x))
		for k, v2 := range x {
			m2[fmt.Sprint(k)] = toJSONCompatible(v2)
		}
		return m2
	case []interface{}:
		for i, u := range x {
			x[i] = toJSONCompatible(u)
		}
		return x
	default:
		return v
	}
}
