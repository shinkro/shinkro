package database

import (
	"context"
	"database/sql"

	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/shinkro/shinkro/internal/domain"
)

type AnilistAuthRepo struct {
	log zerolog.Logger
	db  *DB
}

func NewAnilistAuthRepo(log zerolog.Logger, db *DB) domain.AnilistAuthRepo {
	return &AnilistAuthRepo{
		log: log,
		db:  db,
	}
}

func (repo *AnilistAuthRepo) Store(ctx context.Context, aa *domain.AnilistAuth) error {
	queryBuilder := repo.db.squirrel.
		Replace("anilistauth").
		Columns("id", "client_id", "client_secret", "redirect_url", "access_token", "token_iv", "oauth_state").
		Values(aa.Id, aa.Config.ClientID, aa.Config.ClientSecret, aa.Config.RedirectURL, aa.AccessToken, aa.TokenIV, aa.OAuthState).
		RunWith(repo.db.handler)

	_, err := queryBuilder.ExecContext(ctx)
	if err != nil {
		repo.log.Err(err).Msg("error executing query")
		return err
	}
	return nil
}

func (repo *AnilistAuthRepo) Get(ctx context.Context) (*domain.AnilistAuth, error) {
	queryBuilder := repo.db.squirrel.
		Select("aa.client_id", "aa.client_secret", "aa.redirect_url", "aa.access_token", "aa.token_iv", "aa.oauth_state").
		From("anilistauth aa").
		Where(sq.Eq{"aa.id": 1}).
		RunWith(repo.db.handler)

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "error building query")
	}

	repo.log.Trace().Str("database", "anilistauth.get").Msgf("query: '%s', args: '%v'", query, args)
	row := repo.db.handler.QueryRowContext(ctx, query, args...)

	if err := row.Err(); err != nil {
		return nil, errors.Wrap(err, "error rows get anilistauth")
	}

	var clientId, clientSecret, redirectURL, oauthState string
	var accessToken, tokenIV []byte

	if err := row.Scan(&clientId, &clientSecret, &redirectURL, &accessToken, &tokenIV, &oauthState); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, errors.Wrap(err, "error scanning row")
	}

	aa := domain.NewAnilistAuth(clientId, clientSecret, redirectURL, accessToken, tokenIV)
	aa.OAuthState = oauthState
	return aa, nil
}

func (repo *AnilistAuthRepo) Delete(ctx context.Context) error {
	queryBuilder := repo.db.squirrel.
		Delete("anilistauth").
		Where(sq.Eq{"id": 1})

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return errors.Wrap(err, "error building delete query")
	}

	repo.log.Trace().Str("database", "anilistauth.delete").Msgf("query: '%s', args: '%v'", query, args)

	_, err = repo.db.handler.ExecContext(ctx, query, args...)
	if err != nil {
		return errors.Wrap(err, "error executing delete query")
	}
	return nil
}
