package syncinspector

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/shinkro/shinkro/internal/domain"
)

func (s *Service) catalogue(ctx context.Context, provider string, key int) (int, int, string, error) {
	columns := map[string]string{"anidb": "anidb_id", "tvdb": "tvdb_id", "tmdb": "tmdb_id", "myanimelist": "mal_id"}
	column, ok := columns[provider]
	if !ok {
		return 0, 0, "", fail("INVALID_INPUT", "Unsupported source provider.")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT mal_id,COALESCE(tvdb_id,0),COALESCE(title,''),COALESCE(type,'') FROM anime WHERE `+column+`=? ORDER BY CASE WHEN tvdb_id>0 THEN 0 ELSE 1 END,mal_id DESC`, key)
	if err != nil {
		return 0, 0, "", err
	}
	defer rows.Close()
	type candidate struct {
		mid, tv     int
		title, kind string
	}
	all := []candidate{}
	tvOnly := []candidate{}
	for rows.Next() {
		var c candidate
		if err = rows.Scan(&c.mid, &c.tv, &c.title, &c.kind); err != nil {
			return 0, 0, "", err
		}
		all = append(all, c)
		if c.kind == "tv" {
			tvOnly = append(tvOnly, c)
		}
	}
	if err = rows.Err(); err != nil {
		return 0, 0, "", err
	}
	if len(all) == 0 {
		return 0, 0, "", sql.ErrNoRows
	}
	if len(all) > 1 {
		if len(tvOnly) == 1 {
			return tvOnly[0].mid, tvOnly[0].tv, tvOnly[0].title, nil
		}
		return 0, 0, "", fail("AMBIGUOUS_MAPPING", "More than one MAL title has this provider ID. Choose a destination explicitly.")
	}
	return all[0].mid, all[0].tv, all[0].title, nil
}

func (s *Service) resolve(ctx context.Context, in Input, c *Correction) (Resolution, error) {
	r := Resolution{Episode: in.Episode, Steps: []Step{{Label: "Received", Detail: fmt.Sprintf("%s %d · season %d episode %d", strings.ToUpper(in.Source), in.SourceID, in.Season, in.Episode)}}}
	complete := func(mid int, episode int, origin, title string, rule interface{}) (Resolution, error) {
		r.MALID = mid
		r.Episode = episode
		r.Origin = origin
		r.Title = title
		r.Rule = rule
		// Freshness and unrelated source edits explain the observation without
		// invalidating an otherwise identical approved destination and patch.
		decision := r
		decision.Sources = nil
		r.Fingerprint = hash(struct {
			Input      Input
			Resolution Resolution
		}{in, decision})
		if mid < 1 || episode < 1 && in.Event == "media.scrobble" {
			return r, fail("INVALID_EPISODE", "The destination or converted episode is invalid.")
		}
		return r, nil
	}
	if c != nil {
		if c.MALID < 1 || c.Episode < 1 {
			return r, fail("INVALID_INPUT", "Choose a MAL title and a positive destination episode.")
		}
		r.Steps = append(r.Steps, Step{"Event correction", "Use the explicitly selected destination for this event only."})
		return complete(c.MALID, c.Episode, "event_correction", "", c)
	}
	overrides, err := s.Overrides(ctx)
	if err != nil {
		return r, err
	}
	var chosen *Override
	for _, o := range overrides {
		if scope(o, in) && in.Episode >= o.SourceEpisodeStart && in.Episode <= o.SourceEpisodeEnd {
			if chosen != nil {
				return r, fail("AMBIGUOUS_MAPPING", "Local rules overlap; remove the conflict before syncing.")
			}
			copy := o
			chosen = &copy
		}
	}
	if chosen != nil {
		r.Steps = append(r.Steps, Step{"Local override", fmt.Sprintf("Source episodes %d–%d → MAL %d starting at episode %d", chosen.SourceEpisodeStart, chosen.SourceEpisodeEnd, chosen.MALID, chosen.MALEpisodeStart)})
		return complete(chosen.MALID, chosen.MALEpisodeStart+in.Episode-chosen.SourceEpisodeStart, "local_override", "", chosen)
	}
	if in.Source == "myanimelist" {
		r.Steps = append(r.Steps, Step{"Direct MAL", "Plex supplied the MAL destination directly."})
		return complete(in.SourceID, in.Episode, "direct_mal", "", nil)
	}
	provider, key := in.Source, in.SourceID
	catalogueCaptured := false
	captureCatalogue := func() error {
		if catalogueCaptured {
			return nil
		}
		snapshot, err := s.catalogueSource(ctx)
		if err != nil {
			return fail("SOURCE_UNAVAILABLE", "The catalogue snapshot could not be recorded. No changes were made.")
		}
		r.Sources = append(r.Sources, snapshot)
		catalogueCaptured = true
		return nil
	}
	if in.Source == "anidb" {
		if err := captureCatalogue(); err != nil {
			return r, err
		}
		mid, tv, title, e := s.catalogue(ctx, in.Source, in.SourceID)
		if e == nil && in.Season == 1 {
			r.Steps = append(r.Steps, Step{"Canonical AniDB", fmt.Sprintf("AniDB %d → MAL %d without a custom mapping", in.SourceID, mid)})
			return complete(mid, in.Episode, "canonical_anidb", title, nil)
		}
		if _, ok := e.(*Error); ok {
			return r, e
		}
		if e == nil && tv > 0 {
			provider, key = "tvdb", tv
			r.Steps = append(r.Steps, Step{"TVDB conversion", fmt.Sprintf("AniDB %d → TVDB %d for season %d", in.SourceID, key, in.Season)})
		}
	}
	if provider == "tvdb" || provider == "tmdb" {
		maps, settings, e := s.maps.Snapshot(ctx)
		if e != nil {
			return r, fail("SOURCE_UNAVAILABLE", "Mapping sources are unavailable. Refresh sources and try again.")
		}
		origin := "community_rule"
		_, tvErr := os.Stat(settings.CustomMapTVDBPath)
		_, tmErr := os.Stat(settings.CustomMapTMDBPath)
		if provider == "tvdb" && settings.TVDBEnabled && tvErr == nil || provider == "tmdb" && settings.TMDBEnabled && tmErr == nil {
			origin = "custom_file"
		}
		label := "Community " + strings.ToUpper(provider) + " mapping"
		if origin == "custom_file" {
			label = "Custom " + strings.ToUpper(provider) + " mapping"
		}
		var content interface{}
		entries := 0
		if provider == "tvdb" && maps.AnimeTVShows != nil {
			content, entries = maps.AnimeTVShows, len(maps.AnimeTVShows.Anime)
		} else if provider == "tmdb" && maps.AnimeMovies != nil {
			content, entries = maps.AnimeMovies, len(maps.AnimeMovies.AnimeMovie)
		} else {
			return r, fail("SOURCE_UNAVAILABLE", "The selected mapping snapshot is incomplete.")
		}
		snapshot, snapshotErr := s.recordSource(ctx, provider, label, entries, content)
		if snapshotErr != nil {
			return r, fail("SOURCE_UNAVAILABLE", "The mapping snapshot could not be recorded. No changes were made.")
		}
		r.Sources = append(r.Sources, snapshot)
		var d *domain.AnimeMapDetails
		code := "MAPPING_NOT_FOUND"
		if in.MediaType == "movie" && provider == "tmdb" {
			for _, m := range maps.AnimeMovies.AnimeMovie {
				if m.TMDBID == key {
					if d != nil && d.Malid != m.MALID {
						return r, fail("AMBIGUOUS_MAPPING", "Multiple movie rules select different MAL destinations.")
					}
					d = &domain.AnimeMapDetails{Malid: m.MALID}
				}
			}
		} else if in.MediaType == "episode" && provider == "tvdb" {
			d, code = maps.AnimeTVShows.ResolveStrict(key, in.Season, in.Episode)
		}
		if d != nil {
			ep := in.Episode
			if in.Event == "media.scrobble" && in.MediaType != "movie" {
				ep = d.CalculateEpNum(in.Episode)
			}
			r.Steps = append(r.Steps, Step{"Mapping", fmt.Sprintf("%s %d season %d episode %d → MAL %d episode %d", strings.ToUpper(provider), key, in.Season, in.Episode, d.Malid, ep)})
			return complete(d.Malid, ep, origin, "", d)
		}
		if code != "MAPPING_NOT_FOUND" {
			return r, fail(code, "The source episode is undefined, invalid, or conflicts with another mapping. Choose an explicit correction.")
		}
	}
	if in.Season == 1 {
		if err := captureCatalogue(); err != nil {
			return r, err
		}
		mid, _, title, e := s.catalogue(ctx, provider, key)
		if e == nil {
			r.Steps = append(r.Steps, Step{"Catalogue fallback", fmt.Sprintf("%s %d → MAL %d", strings.ToUpper(provider), key, mid)})
			return complete(mid, in.Episode, "catalogue", title, nil)
		}
		if _, ok := e.(*Error); ok {
			return r, e
		}
	}
	return r, fail("MAPPING_NOT_FOUND", fmt.Sprintf("No MAL destination was found for %s %d, season %d. Refresh sources or select a MAL title.", strings.ToUpper(in.Source), in.SourceID, in.Season))
}
