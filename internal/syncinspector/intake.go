package syncinspector

import (
	"context"
	"encoding/json"
	"time"
)

const DuplicateWindow = 30 * time.Second

// The key is source identity, never a title or a processed MAL episode. Keeping
// the window bounded permits a later intentional watch/rating of the same item.
func sourceEventKey(in Input) string {
	return hash(struct {
		Server, Account, Library, Provider, Media, Event string
		SourceID, Season, Episode                        int
		Rating                                           float32
	}{in.ServerID, in.Account, in.Library, in.Source, in.MediaType, in.Event, in.SourceID, in.Season, in.Episode, in.Rating})
}
func (s *Service) recentDuplicate(ctx context.Context, in Input, account string, now time.Time) (int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT i.input,j.payload FROM sync_job j JOIN sync_input i ON i.event_id=j.event_id WHERE j.kind='live' AND j.created_at>=? ORDER BY j.created_at ASC,j.id ASC`, now.Add(-DuplicateWindow))
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	key := sourceEventKey(in)
	for rows.Next() {
		var rawInput, rawJob string
		var prior Input
		var j Job
		if err = rows.Scan(&rawInput, &rawJob); err != nil {
			return 0, err
		}
		if err = json.Unmarshal([]byte(rawInput), &prior); err != nil {
			return 0, err
		}
		if err = json.Unmarshal([]byte(rawJob), &j); err != nil {
			return 0, err
		}
		if j.Code == "DUPLICATE_SOURCE_EVENT" || j.AccountID != account || j.EventID == in.EventID {
			continue
		}
		if j.Status != "queued" && j.Status != "processing" && j.Status != "synced" && j.Status != "no_change" && j.Code != "UNCERTAIN_WRITE" {
			continue
		}
		if key == sourceEventKey(prior) {
			return prior.EventID, nil
		}
	}
	return 0, rows.Err()
}
