package syncinspector

import (
	"context"
	"strconv"
	"strings"
)

type SearchResult struct {
	MALID         int    `json:"malId"`
	Title         string `json:"title"`
	PictureURL    string `json:"pictureUrl"`
	TotalEpisodes int    `json:"totalEpisodes"`
	MediaType     string `json:"mediaType"`
}

func (s *Service) Search(ctx context.Context, q string) ([]SearchResult, error) {
	q = strings.TrimSpace(q)
	out := []SearchResult{}
	if q == "" {
		return out, nil
	}
	if len(q) > 200 {
		return out, fail("INVALID_INPUT", "Search must be shorter than 200 characters.")
	}
	if mid, err := strconv.Atoi(q); err == nil && mid > 0 {
		v, e := s.mal.Read(ctx, mid)
		if e == nil {
			return []SearchResult{{MALID: mid, Title: v.Title, PictureURL: v.PictureURL, TotalEpisodes: v.TotalEpisodes}}, nil
		}
	}
	rows, err := s.db.QueryContext(ctx, `SELECT mal_id,COALESCE(title,''),COALESCE(type,'') FROM anime WHERE title LIKE ? ESCAPE '\' OR en_title LIKE ? ESCAPE '\' ORDER BY CASE WHEN lower(title)=lower(?) THEN 0 ELSE 1 END,title LIMIT 20`, "%"+escapeLike(q)+"%", "%"+escapeLike(q)+"%", q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var v SearchResult
		if err := rows.Scan(&v.MALID, &v.Title, &v.MediaType); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func escapeLike(q string) string {
	q = strings.ReplaceAll(q, `\`, `\\`)
	q = strings.ReplaceAll(q, "%", `\%`)
	return strings.ReplaceAll(q, "_", `\_`)
}
