package database

import (
	"context"
	"database/sql"
	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/shinkro/shinkro/internal/domain"
)

type MalAuthRepo struct {
	log zerolog.Logger
	db  *DB
}

func NewMalAuthRepo(log zerolog.Logger, db *DB) domain.MalAuthRepo {
	return &MalAuthRepo{
		log: log,
		db:  db,
	}
}

func (repo *MalAuthRepo) Store(ctx context.Context, ma *domain.MalAuth) error {

	queryBuilder := repo.db.squirrel.
		Replace("malauth").
		Columns("id", "client_id", "client_secret", "access_token", "token_iv").
		Values(ma.Id, ma.Config.ClientID, ma.Config.ClientSecret, ma.AccessToken, ma.TokenIV).
		RunWith(repo.db.handler)

	_, err := queryBuilder.ExecContext(ctx)
	if err != nil {
		repo.log.Err(err).Msg("error executing query")
		return err
	}

	return nil
}

func (repo *MalAuthRepo) Get(ctx context.Context) (*domain.MalAuth, error) {
	queryBuilder := repo.db.squirrel.
		Select("ma.client_id", "ma.client_secret", "ma.access_token", "ma.token_iv").
		From("malauth ma").
		Where(sq.Eq{"ma.id": 1}).
		RunWith(repo.db.handler)

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "error building query")
	}

	repo.log.Trace().Str("database", "malauth.get").Msgf("query: '%s', args: '%v'", query, args)
	row := repo.db.handler.QueryRowContext(ctx, query, args...)

	if err := row.Err(); err != nil {
		return nil, errors.Wrap(err, "error rows get malauth")
	}

	var clientId, clientSecret string
	var accessToken, tokenIV []byte

	if err := row.Scan(&clientId, &clientSecret, &accessToken, &tokenIV); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, errors.Wrap(err, "error scanning row")
	}

	ma := domain.NewMalAuth(clientId, clientSecret, accessToken, tokenIV)
	return ma, nil
}

func (repo *MalAuthRepo) Delete(ctx context.Context) error {
	queryBuilder := repo.db.squirrel.
		Delete("malauth").
		Where(sq.Eq{"id": 1})

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		repo.log.Err(err).Msg("error building delete query")
		return errors.Wrap(err, "error building delete query")
	}

	repo.log.Trace().Str("database", "malauth.delete").Msgf("query: '%s', args: '%v'", query, args)

	_, err = repo.db.handler.ExecContext(ctx, query, args...)
	if err != nil {
		repo.log.Err(err).Msg("error executing delete query")
		return errors.Wrap(err, "error executing delete query")
	}

	return nil
}
