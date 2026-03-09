package domain

import (
	"context"
)

type AnimeRepo interface {
	GetByID(ctx context.Context, req *GetAnimeRequest) (*Anime, error)
	StoreMultiple(anime []*Anime) error
}

type Anime struct {
	MALId        int    `json:"malid"`
	MainTitle    string `json:"title"`
	EnglishTitle string `json:"enTitle"`
	AniDBId      int    `json:"anidbid"`
	TVDBId       int    `json:"tvdbid"`
	TMDBId       int    `json:"tmdbid"`
	AnimeType    string `json:"type"`
	ReleaseDate  string `json:"releaseDate"`
}

type AnimeUrl string

const ShinkroDB AnimeUrl = "https://github.com/shinkro/community-mapping/raw/main/shinkrodb/for-shinkro.json"

type GetAnimeRequest struct {
	IDtype PlexSupportedDBs
	Id     int
}
