package database

import (
	"context"
	"database/sql"

	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/shinkro/shinkro/internal/domain"
)

type AnimeRepo struct {
	log zerolog.Logger
	db  *DB
}

func NewAnimeRepo(log zerolog.Logger, db *DB) domain.AnimeRepo {
	return &AnimeRepo{
		log: log,
		db:  db,
	}
}

func (repo *AnimeRepo) GetByID(ctx context.Context, req *domain.GetAnimeRequest) (*domain.Anime, error) {

	// Map PlexSupportedDBs to database column names
	var columnName string
	switch req.IDtype {
	case domain.MAL:
		columnName = "mal_id"
	case domain.TVDB:
		columnName = "tvdb_id"
	case domain.TMDB:
		columnName = "tmdb_id"
	case domain.AniDB:
		columnName = "anidb_id"
	default:
		columnName = string(req.IDtype) + "_id"
	}
	id := "a." + columnName

	queryBuilder := repo.db.squirrel.
		Select("a.mal_id", "a.title", "a.en_title", "a.anidb_id", "a.tvdb_id", "a.tmdb_id", "a.type", "a.releaseDate").
		From("anime a").
		Where(sq.Eq{id: req.Id}).
		OrderBy("CASE WHEN a.tvdb_id > 0 THEN 0 ELSE 1 END", "a.mal_id DESC")

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "error building query")
	}

	repo.log.Trace().Str("database", "anime.getByTVDBID").Msgf("query: '%s', args: '%v'", query, args)

	rows, err := repo.db.handler.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "error executing query")
	}
	defer rows.Close()

	var results []domain.Anime
	for rows.Next() {
		var anime domain.Anime
		var title, enTitle, animeType, releaseDate sql.NullString
		var anidbid, tvdbid, tmdbid sql.NullInt32

		if err := rows.Scan(&anime.MALId, &title, &enTitle, &anidbid, &tvdbid, &tmdbid, &animeType, &releaseDate); err != nil {
			return nil, errors.Wrap(err, "error scanning row")
		}

		anime.MainTitle = title.String
		anime.EnglishTitle = enTitle.String
		anime.AnimeType = animeType.String
		anime.ReleaseDate = releaseDate.String
		anime.AniDBId = int(anidbid.Int32)
		anime.TVDBId = int(tvdbid.Int32)
		anime.TMDBId = int(tmdbid.Int32)

		results = append(results, anime)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error iterating rows")
	}

	if len(results) == 0 {
		return nil, sql.ErrNoRows
	}

	anime := results[0]

	if len(results) > 1 {
		// Try to tie-break by type: if there's exactly one with type = 'tv', use that one
		var tvResults []domain.Anime
		for _, r := range results {
			if r.AnimeType == "tv" {
				tvResults = append(tvResults, r)
			}
		}

		if len(tvResults) == 1 {
			anime = tvResults[0]
			repo.log.Debug().Int("tvdbId", req.Id).Int("rowCount", len(results)).Msg("Multiple rows found, using the single 'tv' type row")
		} else {
			anime.MALId = 0
			repo.log.Debug().Int("tvdbId", req.Id).Int("rowCount", len(results)).Msg("Multiple rows found, setting MAL ID to 0")
		}
	}

	return &anime, nil
}

func (repo *AnimeRepo) StoreMultiple(anime []*domain.Anime) error {
	tx, err := repo.db.handler.Begin()
	if err != nil {
		return err
	}

	for _, a := range anime {
		queryBuilder := repo.db.squirrel.
			Replace("anime").
			Columns("mal_id", "title", "en_title", "anidb_id", "tvdb_id", "tmdb_id", "type", "releaseDate").
			Values(a.MALId, a.MainTitle, a.EnglishTitle, a.AniDBId, a.TVDBId, a.TMDBId, a.AnimeType, a.ReleaseDate).
			RunWith(tx)

		_, err := queryBuilder.Exec()
		if err != nil {
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}
