package domain

import "context"

type MappingRepo interface {
	Store(ctx context.Context, m *MapSettings) error
	Get(ctx context.Context) (*MapSettings, error)
}

type Mapping struct {
	MapSettings *MapSettings
	AnimeMap    *AnimeMap
}

type MapSettings struct {
	TVDBEnabled       bool   `json:"tvdb_enabled"`
	TMDBEnabled       bool   `json:"tmdb_enabled"`
	CustomMapTVDBPath string `json:"tvdb_path"`
	CustomMapTMDBPath string `json:"tmdb_path"`
}

type AnimeMap struct {
	AnimeTVShows *AnimeTVShows
	AnimeMovies  *AnimeMovies
}

type AnimeMapDetails struct {
	Malid            int
	Start            int
	UseMapping       bool
	MappingType      string      // "explicit" or "range" (default)
	ExplicitEpisodes map[int]int // tvdbEp -> malEp for explicit mappings
	SkipMalEpisodes  []int       // MAL episodes to skip for range mappings
}

type AnimeTVShows struct {
	Anime []AnimeTV `yaml:"AnimeMap" json:"AnimeMap"`
}

type AnimeTV struct {
	Malid        int            `yaml:"malid" json:"malid"`
	Title        string         `yaml:"title" json:"title"`
	Type         string         `yaml:"type" json:"type"`
	Tvdbid       int            `yaml:"tvdbid" json:"tvdbid"`
	TvdbSeason   int            `yaml:"tvdbseason" json:"tvdbseason"`
	Start        int            `yaml:"start" json:"start"`
	UseMapping   bool           `yaml:"useMapping" json:"useMapping"`
	AnimeMapping []AnimeMapping `yaml:"animeMapping" json:"animeMapping"`
	// Temporary fields for mapping data (populated during lookup)
	MappingType      string      `yaml:"-" json:"-"` // Not serialized
	ExplicitEpisodes map[int]int `yaml:"-" json:"-"` // Not serialized
	SkipMalEpisodes  []int       `yaml:"-" json:"-"` // Not serialized
}

type AnimeMapping struct {
	TvdbSeason       int         `yaml:"tvdbseason" json:"tvdbseason"`
	Start            int         `yaml:"start" json:"start"`
	MappingType      string      `yaml:"mappingType,omitempty" json:"mappingType,omitempty"`           // "explicit" or "range" (default)
	ExplicitEpisodes map[int]int `yaml:"explicitEpisodes,omitempty" json:"explicitEpisodes,omitempty"` // tvdbEp -> malEp
	SkipMalEpisodes  []int       `yaml:"skipMalEpisodes,omitempty" json:"skipMalEpisodes,omitempty"`   // MAL episodes to skip
}

type AnimeMovies struct {
	AnimeMovie []AnimeMovie `yaml:"animeMovies" json:"animeMovies"`
}

type AnimeMovie struct {
	MainTitle string `yaml:"mainTitle" json:"mainTitle"`
	TMDBID    int    `yaml:"tmdbid" json:"tmdbid"`
	MALID     int    `yaml:"malid" json:"malid"`
}

type CommunityMapUrls string

const (
	CommunityMapTVDB CommunityMapUrls = "https://github.com/shinkro/community-mapping/raw/main/tvdb-mal.yaml"
	CommunityMapTMDB CommunityMapUrls = "https://github.com/shinkro/community-mapping/raw/main/tmdb-mal.yaml"
)

type MapValidationUrls string

const (
	TVDBValidationUrl MapValidationUrls = "https://github.com/shinkro/community-mapping/raw/main/.github/schema-tvdb.json"
	TMDBValidationUrl MapValidationUrls = "https://github.com/shinkro/community-mapping/raw/main/.github/schema-tmdb.json"
)

func (s *AnimeTVShows) CheckMap(tvdbid, tvdbseason, ep int) (bool, *AnimeTV) {
	candidates := s.findMatchingAnime(tvdbid, tvdbseason)
	if len(candidates) == 1 {
		return true, &candidates[0]
	} else if len(candidates) > 1 {
		anime := s.findBestMatchingAnime(ep, candidates)
		return true, &anime
	}

	return false, nil
}

func (am *AnimeMovies) CheckMap(tmdbid int) (bool, *AnimeMovie) {
	for _, animeMovie := range am.AnimeMovie {
		if animeMovie.TMDBID == tmdbid {
			return true, &animeMovie
		}
	}

	return false, nil
}

func (s *AnimeTVShows) findMatchingAnime(tvdbid, tvdbseason int) []AnimeTV {
	var matchingAnime []AnimeTV
	var mappedAnime []AnimeTV

	for _, anime := range s.Anime {
		if tvdbid != anime.Tvdbid {
			continue
		}

		if !anime.UseMapping && tvdbseason == anime.TvdbSeason {
			matchingAnime = append(matchingAnime, anime)
			continue
		}

		matchingMappedAnime := s.findMatchingMappedAnime(anime, tvdbseason)
		if matchingMappedAnime != nil {
			mappedAnime = append(mappedAnime, *matchingMappedAnime)
		}
	}

	if len(mappedAnime) > 0 {
		if len(mappedAnime) == 1 && len(matchingAnime) > 0 {
			var allCandidates []AnimeTV
			allCandidates = append(allCandidates, matchingAnime...)
			allCandidates = append(allCandidates, mappedAnime...)
			return allCandidates
		} else {
			return mappedAnime
		}
	}

	return matchingAnime
}

func (s *AnimeTVShows) findMatchingMappedAnime(anime AnimeTV, tvdbseason int) *AnimeTV {
	if !anime.UseMapping {
		return nil
	}

	for _, animeMap := range anime.AnimeMapping {
		if tvdbseason == animeMap.TvdbSeason {
			anime.TvdbSeason = animeMap.TvdbSeason
			anime.Start = animeMap.Start
			// Store mapping data for later use
			anime.MappingType = animeMap.MappingType
			anime.ExplicitEpisodes = animeMap.ExplicitEpisodes
			anime.SkipMalEpisodes = animeMap.SkipMalEpisodes
			return &anime
		}
	}

	return nil
}

func (s *AnimeTVShows) findBestMatchingAnime(ep int, candidates []AnimeTV) AnimeTV {
	var anime AnimeTV
	largestStart := -1
	var fallbackAnime AnimeTV
	largestFallbackStart := -1

	for _, v := range candidates {
		if ep >= v.Start && v.Start > largestStart {
			largestStart = v.Start
			anime = v
		}

		if v.Start > largestFallbackStart {
			largestFallbackStart = v.Start
			fallbackAnime = v
		}
	}

	if anime.Malid == 0 {
		anime = fallbackAnime
	}

	return anime
}

func (ad *AnimeMapDetails) CalculateEpNum(oldEpNum int) int {
	if ad.Start == 0 {
		ad.Start = 1
	}

	// Handle explicit episode mappings
	if ad.MappingType == "explicit" && ad.ExplicitEpisodes != nil {
		if malEp, found := ad.ExplicitEpisodes[oldEpNum]; found {
			return malEp
		}
		// Explicit mapping but episode not found - fall through to default behavior
	}

	// Handle range mappings with skip logic
	if len(ad.SkipMalEpisodes) > 0 {
		position := oldEpNum
		malEp := ad.Start - 1 // Start one before, so first iteration increments to Start
		count := 0

		for count < position {
			malEp++
			// Check if this MAL episode should be skipped
			shouldSkip := false
			for _, skip := range ad.SkipMalEpisodes {
				if malEp == skip {
					shouldSkip = true
					break
				}
			}
			if !shouldSkip {
				count++
			}
		}
		return malEp
	}

	// Default range mapping (no skips)
	if ad.UseMapping {
		return ad.Start + oldEpNum - 1
	}

	return oldEpNum - ad.Start + 1
}

func NewMapSettings(tvdb, tmdb bool, tvdbPath, tmdbPath string) *MapSettings {
	return &MapSettings{
		TVDBEnabled:       tvdb,
		TMDBEnabled:       tmdb,
		CustomMapTVDBPath: tvdbPath,
		CustomMapTMDBPath: tmdbPath,
	}
}

// ShouldLoadLocal returns whether local maps should be loaded based on settings.
// Actual file existence check should be done by the service layer.
func (ms *MapSettings) ShouldLoadLocal() (bool, bool) {
	tvdb := ms.TVDBEnabled && ms.CustomMapTVDBPath != ""
	tmdb := ms.TMDBEnabled && ms.CustomMapTMDBPath != ""
	return tvdb, tmdb
}

