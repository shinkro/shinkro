package domain

import (
	"context"

	"golang.org/x/oauth2"
)

type AnilistAuthRepo interface {
	Store(ctx context.Context, aa *AnilistAuth) error
	Get(ctx context.Context) (*AnilistAuth, error)
	Delete(ctx context.Context) error
}

type AnilistAuth struct {
	Id          int
	Config      oauth2.Config
	AccessToken []byte
	TokenIV     []byte
	OAuthState  string // stored temporarily during OAuth flow, cleared after callback
}

const AnilistAuthURL = "https://anilist.co/api/v2/oauth/authorize"
const AnilistTokenURL = "https://anilist.co/api/v2/oauth/token"
const AnilistGraphQLURL = "https://graphql.anilist.co"

func NewAnilistAuth(clientID, clientSecret, redirectURL string, accessToken, tokenIV []byte) *AnilistAuth {
	return &AnilistAuth{
		Id: 1,
		Config: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint: oauth2.Endpoint{
				AuthURL:   AnilistAuthURL,
				TokenURL:  AnilistTokenURL,
				AuthStyle: oauth2.AuthStyleInParams,
			},
		},
		AccessToken: accessToken,
		TokenIV:     tokenIV,
	}
}
