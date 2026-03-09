package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/shinkro/shinkro/internal/domain"
)

type AnimeUpdateRepo struct {
	log zerolog.Logger
	db  *DB
}

func NewAnimeUpdateRepo(log zerolog.Logger, db *DB) domain.AnimeUpdateRepo {
	return &AnimeUpdateRepo{
		log: log.With().Str("repo", "animeupdate").Logger(),
		db:  db,
	}
}

func (repo *AnimeUpdateRepo) Store(ctx context.Context, r *domain.AnimeUpdate) error {
	listDetails, err := json.Marshal(r.ListDetails)
	if err != nil {
		return errors.Wrap(err, "failed to marshal listDetails")
	}

	listStatus, err := json.Marshal(r.ListStatus)
	if err != nil {
		return errors.Wrap(err, "failed to marshal listStatus")
	}

	queryBuilder := repo.db.squirrel.
		Insert("anime_update").
		Columns("mal_id", "source_db", "source_id", "episode_num", "season_num", "time_stamp", "list_details", "list_status", "plex_id", "status", "error_type", "error_message").
		Values(r.MALId, r.SourceDB, r.SourceId, r.EpisodeNum, r.SeasonNum, r.Timestamp, string(listDetails), string(listStatus), r.PlexId, r.Status, r.ErrorType, r.ErrorMessage).
		Suffix("RETURNING id").RunWith(repo.db.handler)

	var retID int64

	if err := queryBuilder.QueryRowContext(ctx).Scan(&retID); err != nil {
		repo.log.Debug().Err(err).Msg("error executing query")
		return errors.Wrap(err, "error executing query")
	}

	r.ID = retID
	repo.log.Debug().Msgf("animeUpdate.store: %+v", r)
	return nil
}

func (repo *AnimeUpdateRepo) GetByID(ctx context.Context, req *domain.GetAnimeUpdateRequest) (*domain.AnimeUpdate, error) {
	return nil, nil
}

func (repo *AnimeUpdateRepo) Count(ctx context.Context) (int, error) {
	queryBuilder := repo.db.squirrel.
		Select("count(*)").
		Where(sq.Eq{"status": string(domain.AnimeUpdateStatusSuccess)}).
		From("anime_update")

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "error building query")
	}

	row := repo.db.handler.QueryRowContext(ctx, query, args...)
	if err := row.Err(); err != nil {
		return 0, errors.Wrap(err, "error executing query")
	}

	var count int
	if err := row.Scan(&count); err != nil {
		return 0, errors.Wrap(err, "error scanning row")
	}

	return count, nil
}

func (repo *AnimeUpdateRepo) GetRecentUnique(ctx context.Context, limit int) ([]*domain.AnimeUpdate, error) {
	latest := repo.db.squirrel.
		Select("mal_id, MAX(time_stamp) AS max_ts").
		From("anime_update").
		Where(sq.Eq{"status": string(domain.AnimeUpdateStatusSuccess)}).
		GroupBy("mal_id")

	queryBuilder := repo.db.squirrel.
		Select("au.id, au.mal_id, au.source_db, au.source_id, au.episode_num, au.season_num, au.time_stamp, au.list_details, au.list_status, au.plex_id, au.status, au.error_type, au.error_message").
		FromSelect(latest, "latest").
		Join("anime_update au ON latest.mal_id = au.mal_id AND latest.max_ts = au.time_stamp").
		Where(sq.Eq{"au.status": string(domain.AnimeUpdateStatusSuccess)}).
		OrderBy("au.time_stamp DESC").
		Limit(uint64(limit))

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "error building query")
	}

	rows, err := repo.db.handler.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "error executing query")
	}
	defer rows.Close()

	updates := make([]*domain.AnimeUpdate, 0)
	for rows.Next() {
		var au domain.AnimeUpdate
		var listDetailsBytes, listStatusBytes []byte
		var status, errorType, errorMessage sql.NullString
		if err := rows.Scan(&au.ID, &au.MALId, &au.SourceDB, &au.SourceId, &au.EpisodeNum, &au.SeasonNum, &au.Timestamp, &listDetailsBytes, &listStatusBytes, &au.PlexId, &status, &errorType, &errorMessage); err != nil {
			return nil, errors.Wrap(err, "error scanning row")
		}
		if err := json.Unmarshal(listDetailsBytes, &au.ListDetails); err != nil {
			return nil, errors.Wrap(err, "error unmarshalling list_details")
		}
		if err := json.Unmarshal(listStatusBytes, &au.ListStatus); err != nil {
			return nil, errors.Wrap(err, "error unmarshalling list_status")
		}
		if status.Valid {
			au.Status = domain.AnimeUpdateStatusType(status.String)
		}
		if errorType.Valid {
			au.ErrorType = domain.AnimeUpdateErrorType(errorType.String)
		}
		if errorMessage.Valid {
			au.ErrorMessage = errorMessage.String
		}
		updates = append(updates, &au)
	}
	return updates, nil
}

func (repo *AnimeUpdateRepo) GetByPlexID(ctx context.Context, plexID int64) (*domain.AnimeUpdate, error) {
	queryBuilder := repo.db.squirrel.
		Select("id, mal_id, source_db, source_id, episode_num, season_num, time_stamp, list_details, list_status, plex_id, status, error_type, error_message").
		From("anime_update").
		Where("plex_id = ?", plexID).
		OrderBy("time_stamp DESC").
		Limit(1)

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "error building query")
	}

	row := repo.db.handler.QueryRowContext(ctx, query, args...)
	var au domain.AnimeUpdate
	var listDetailsBytes, listStatusBytes []byte
	var status, errorType, errorMessage sql.NullString
	if err := row.Scan(&au.ID, &au.MALId, &au.SourceDB, &au.SourceId, &au.EpisodeNum, &au.SeasonNum, &au.Timestamp, &listDetailsBytes, &listStatusBytes, &au.PlexId, &status, &errorType, &errorMessage); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No update for this plex_id
		}
		return nil, errors.Wrap(err, "error scanning row")
	}
	if err := json.Unmarshal(listDetailsBytes, &au.ListDetails); err != nil {
		return nil, errors.Wrap(err, "error unmarshalling list_details")
	}
	if err := json.Unmarshal(listStatusBytes, &au.ListStatus); err != nil {
		return nil, errors.Wrap(err, "error unmarshalling list_status")
	}
	if status.Valid {
		au.Status = domain.AnimeUpdateStatusType(status.String)
	}
	if errorType.Valid {
		au.ErrorType = domain.AnimeUpdateErrorType(errorType.String)
	}
	if errorMessage.Valid {
		au.ErrorMessage = errorMessage.String
	}
	return &au, nil
}

func (repo *AnimeUpdateRepo) GetByPlexIDs(ctx context.Context, plexIDs []int64) ([]*domain.AnimeUpdate, error) {
	if len(plexIDs) == 0 {
		return []*domain.AnimeUpdate{}, nil
	}

	queryBuilder := repo.db.squirrel.
		Select("id, mal_id, source_db, source_id, episode_num, season_num, time_stamp, list_details, list_status, plex_id, status, error_type, error_message").
		From("anime_update").
		Where(sq.Eq{"plex_id": plexIDs}).
		OrderBy("time_stamp DESC")

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "error building query")
	}

	repo.log.Trace().Str("database", "animeupdate.getByPlexIDs").Msgf("query: '%s', args: '%v'", query, args)

	rows, err := repo.db.handler.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "error executing query")
	}
	defer rows.Close()

	var updates []*domain.AnimeUpdate
	for rows.Next() {
		var au domain.AnimeUpdate
		var listDetailsBytes, listStatusBytes []byte
		var status, errorType, errorMessage sql.NullString
		if err := rows.Scan(&au.ID, &au.MALId, &au.SourceDB, &au.SourceId, &au.EpisodeNum, &au.SeasonNum, &au.Timestamp, &listDetailsBytes, &listStatusBytes, &au.PlexId, &status, &errorType, &errorMessage); err != nil {
			return nil, errors.Wrap(err, "error scanning row")
		}
		if err := json.Unmarshal(listDetailsBytes, &au.ListDetails); err != nil {
			return nil, errors.Wrap(err, "error unmarshalling list_details")
		}
		if err := json.Unmarshal(listStatusBytes, &au.ListStatus); err != nil {
			return nil, errors.Wrap(err, "error unmarshalling list_status")
		}
		if status.Valid {
			au.Status = domain.AnimeUpdateStatusType(status.String)
		}
		if errorType.Valid {
			au.ErrorType = domain.AnimeUpdateErrorType(errorType.String)
		}
		if errorMessage.Valid {
			au.ErrorMessage = errorMessage.String
		}
		updates = append(updates, &au)
	}

	return updates, nil
}

func (repo *AnimeUpdateRepo) FindAllWithFilters(ctx context.Context, params domain.AnimeUpdateQueryParams) (*domain.FindAnimeUpdatesResponse, error) {
	whereQueryBuilder := sq.And{}

	// Parse search query - check for SourceDB:ID pattern
	if params.Search != "" {
		search := strings.TrimSpace(params.Search)

		// Check if search contains ":" pattern (e.g., "TVDB:12345")
		if strings.Contains(search, ":") {
			parts := strings.SplitN(search, ":", 2)
			if len(parts) == 2 {
				sourceDB := strings.ToUpper(strings.TrimSpace(parts[0]))
				sourceIDStr := strings.TrimSpace(parts[1])

				// Convert sourceDB to match database values
				var dbSourceDB string
				switch sourceDB {
				case "TVDB":
					dbSourceDB = "tvdb"
				case "TMDB":
					dbSourceDB = "tmdb"
				case "ANIDB":
					dbSourceDB = "anidb"
				case "MAL", "MYANIMELIST":
					dbSourceDB = "myanimelist"
				default:
					// If not a recognized source, treat as regular search
					searchPattern := "%" + search + "%"
					whereQueryBuilder = append(whereQueryBuilder, sq.Or{
						sq.Like{"json_extract(au.list_details, '$.title')": searchPattern},
						sq.Eq{"au.mal_id": parseMALID(search)},
					})
				}

				// If we have a valid sourceDB, filter by it
				if dbSourceDB != "" {
					if sourceID, err := strconv.Atoi(sourceIDStr); err == nil {
						whereQueryBuilder = append(whereQueryBuilder, sq.And{
							sq.Eq{"au.source_db": dbSourceDB},
							sq.Eq{"au.source_id": sourceID},
						})
					}
				}
			}
		} else {
			// Regular search - search in title or MAL ID
			searchPattern := "%" + search + "%"
			malID := parseMALID(search)

			searchConditions := sq.Or{}
			searchConditions = append(searchConditions, sq.Like{"json_extract(au.list_details, '$.title')": searchPattern})

			if malID > 0 {
				searchConditions = append(searchConditions, sq.Eq{"au.mal_id": malID})
			}

			whereQueryBuilder = append(whereQueryBuilder, searchConditions)
		}
	}

	// Apply filters
	if params.Filters.Status != "" {
		whereQueryBuilder = append(whereQueryBuilder, sq.Eq{"au.status": string(params.Filters.Status)})
	}

	if params.Filters.ErrorType != "" {
		whereQueryBuilder = append(whereQueryBuilder, sq.Eq{"au.error_type": string(params.Filters.ErrorType)})
	}

	if params.Filters.Source != "" {
		whereQueryBuilder = append(whereQueryBuilder, sq.Eq{"au.source_db": string(params.Filters.Source)})
	}

	// Build count query
	countQueryBuilder := repo.db.squirrel.
		Select("COUNT(*)").
		From("anime_update au")

	if len(whereQueryBuilder) > 0 {
		countQueryBuilder = countQueryBuilder.Where(whereQueryBuilder)
	}

	countQuery, countArgs, err := countQueryBuilder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "error building count query")
	}

	// Build main query
	queryBuilder := repo.db.squirrel.
		Select(
			"au.id", "au.mal_id", "au.source_db", "au.source_id", "au.episode_num", "au.season_num",
			"au.time_stamp", "au.list_details", "au.list_status", "au.plex_id",
			"au.status", "au.error_type", "au.error_message",
		).
		From("anime_update au").
		OrderBy("au.id DESC")

	if params.Limit > 0 {
		queryBuilder = queryBuilder.Limit(params.Limit)
	} else {
		queryBuilder = queryBuilder.Limit(20)
	}

	if params.Offset > 0 {
		queryBuilder = queryBuilder.Offset(params.Offset)
	}

	if len(whereQueryBuilder) > 0 {
		queryBuilder = queryBuilder.Where(whereQueryBuilder)
	}

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "error building query")
	}

	// Execute count query
	var totalCount int
	row := repo.db.handler.QueryRowContext(ctx, countQuery, countArgs...)
	if err := row.Scan(&totalCount); err != nil {
		return nil, errors.Wrap(err, "error scanning count")
	}

	repo.log.Trace().Str("database", "animeupdate.findAllWithFilters").Msgf("query: '%s', args: '%v'", query, args)

	resp := &domain.FindAnimeUpdatesResponse{
		Data:       make([]domain.AnimeUpdateListItem, 0),
		TotalCount: totalCount,
	}

	rows, err := repo.db.handler.QueryContext(ctx, query, args...)
	if err != nil {
		return resp, errors.Wrap(err, "error executing query")
	}
	defer rows.Close()

	for rows.Next() {
		var au domain.AnimeUpdate
		var listDetailsBytes, listStatusBytes []byte
		var status, errorType, errorMessage sql.NullString

		if err := rows.Scan(
			&au.ID, &au.MALId, &au.SourceDB, &au.SourceId, &au.EpisodeNum, &au.SeasonNum,
			&au.Timestamp, &listDetailsBytes, &listStatusBytes, &au.PlexId,
			&status, &errorType, &errorMessage,
		); err != nil {
			return resp, errors.Wrap(err, "error scanning row")
		}

		if err := json.Unmarshal(listDetailsBytes, &au.ListDetails); err != nil {
			return resp, errors.Wrap(err, "error unmarshalling list_details")
		}
		if err := json.Unmarshal(listStatusBytes, &au.ListStatus); err != nil {
			return resp, errors.Wrap(err, "error unmarshalling list_status")
		}

		if status.Valid {
			au.Status = domain.AnimeUpdateStatusType(status.String)
		}
		if errorType.Valid {
			au.ErrorType = domain.AnimeUpdateErrorType(errorType.String)
		}
		if errorMessage.Valid {
			au.ErrorMessage = errorMessage.String
		}

		resp.Data = append(resp.Data, domain.AnimeUpdateListItem{
			AnimeUpdate: &au,
		})
	}

	return resp, nil
}

// parseMALID attempts to parse a string as MAL ID (numeric)
func parseMALID(s string) int {
	id, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return id
}
