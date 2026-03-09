package database

import (
	"context"
	"database/sql"
	"github.com/lib/pq"

	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/shinkro/shinkro/internal/domain"
)

type PlexSettingsRepo struct {
	log zerolog.Logger
	db  *DB
}

func NewPlexSettingsRepo(log zerolog.Logger, db *DB) domain.PlexSettingsRepo {
	return &PlexSettingsRepo{
		log: log.With().Str("repo", "plex_settings").Logger(),
		db:  db,
	}
}

func (repo *PlexSettingsRepo) Store(ctx context.Context, ps domain.PlexSettings) (*domain.PlexSettings, error) {

	queryBuilder := repo.db.squirrel.
		Replace("plex_settings").
		Columns("id", "host", "port", "tls", "tls_skip_verify", "token", "token_iv", "username", "anime_libraries", "plex_client_enabled", "client_id").
		Values(1, ps.Host, ps.Port, ps.TLS, ps.TLSSkip, ps.Token, ps.TokenIV, ps.PlexUser, pq.Array(ps.AnimeLibraries), ps.PlexClientEnabled, ps.ClientID).
		RunWith(repo.db.handler)

	_, err := queryBuilder.ExecContext(ctx)
	if err != nil {
		repo.log.Err(err).Msg("error executing query")
		return nil, err
	}

	return &ps, nil
}

func (repo *PlexSettingsRepo) Update(ctx context.Context, ps domain.PlexSettings) (*domain.PlexSettings, error) {
	queryBuilder := repo.db.squirrel.
		Update("plex_settings").
		Where(sq.Eq{"id": 1})

	if ps.Host != "" {
		queryBuilder = queryBuilder.Set("host", ps.Host)
	}
	if ps.Port != 0 {
		queryBuilder = queryBuilder.Set("port", ps.Port)
	}

	queryBuilder = queryBuilder.Set("tls", ps.TLS)
	queryBuilder = queryBuilder.Set("tls_skip_verify", ps.TLSSkip)

	if len(ps.Token) > 0 {
		queryBuilder = queryBuilder.Set("token", ps.Token)
	}
	if len(ps.TokenIV) > 0 {
		queryBuilder = queryBuilder.Set("token_iv", ps.TokenIV)
	}
	if ps.PlexUser != "" {
		queryBuilder = queryBuilder.Set("username", ps.PlexUser)
	}
	if len(ps.AnimeLibraries) > 0 {
		queryBuilder = queryBuilder.Set("anime_libraries", pq.Array(ps.AnimeLibraries))
	}

	queryBuilder = queryBuilder.Set("plex_client_enabled", ps.PlexClientEnabled)

	if ps.ClientID != "" {
		queryBuilder = queryBuilder.Set("client_id", ps.ClientID)
	}

	sqlQuery, args, err := queryBuilder.ToSql()
	if err != nil {
		repo.log.Err(err).Msg("error building update query")
		return nil, errors.Wrap(err, "error building update query")
	}

	repo.log.Trace().Str("database", "plexSettings.update").Msgf("query: '%s', args: '%v'", sqlQuery, args)

	_, err = queryBuilder.RunWith(repo.db.handler).ExecContext(ctx)
	if err != nil {
		repo.log.Err(err).Msg("error executing update query")
		return nil, errors.Wrap(err, "error executing update")
	}

	return &ps, nil
}

func (repo *PlexSettingsRepo) Get(ctx context.Context) (*domain.PlexSettings, error) {
	queryBuilder := repo.db.squirrel.
		Select("ps.host", "ps.port", "ps.tls", "ps.tls_skip_verify", "ps.token", "ps.token_iv", "ps.username", "ps.anime_libraries", "ps.plex_client_enabled", "client_id").
		From("plex_settings ps").
		Where(sq.Eq{"ps.id": 1}).
		RunWith(repo.db.handler)

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "error building query")
	}

	repo.log.Trace().Str("database", "plexSettings.get").Msgf("query: '%s', args: '%v'", query, args)
	row := repo.db.handler.QueryRowContext(ctx, query, args...)

	if err := row.Err(); err != nil {
		return nil, errors.Wrap(err, "error rows get plex settings")
	}

	var host, username, clientID string
	var token, tokenIV []byte
	var port int
	var tls, tls_skip_verify, plex_client_enabled bool
	var anime_libraries []string

	if err := row.Scan(&host, &port, &tls, &tls_skip_verify, &token, &tokenIV, &username, pq.Array(&anime_libraries), &plex_client_enabled, &clientID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, errors.Wrap(err, "error scanning row")
	}

	ps := domain.NewPlexSettings(host, username, clientID, token, tokenIV, port, anime_libraries, plex_client_enabled, tls, tls_skip_verify)

	return ps, nil
}

func (repo *PlexSettingsRepo) Delete(ctx context.Context) error {
	queryBuilder := repo.db.squirrel.Delete("plex_settings").Where(sq.Eq{"id": 1})

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return errors.Wrap(err, "error building query")
	}

	_, err = repo.db.handler.ExecContext(ctx, query, args...)
	if err != nil {
		return errors.Wrap(err, "error executing query")
	}

	repo.log.Debug().Msg("successfully deleted plex settings")

	return nil
}
