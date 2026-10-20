package syncinspector

import (
	"context"
	"encoding/json"
	"strings"
)

// Page in SQL before hydration. Historical tables can grow without making every
// list request read all traces, rules and inputs.
func (s *Service) List(ctx context.Context, limit, offset int, search, status, source, event string) ([]Event, int, error) {
	if limit < 1 || limit > 100 {
		limit = 25
	}
	if offset < 0 {
		offset = 0
	}
	if len(search) > 200 {
		return nil, 0, fail("INVALID_INPUT", "Search must be shorter than 200 characters.")
	}
	base := ` FROM plex_payload p
 LEFT JOIN sync_input i ON i.event_id=p.id
 LEFT JOIN anime_update a ON a.id=(SELECT a2.id FROM anime_update a2 WHERE a2.plex_id=p.id ORDER BY a2.time_stamp DESC,a2.id DESC LIMIT 1)
 LEFT JOIN sync_job j ON j.id=(SELECT j2.id FROM sync_job j2 WHERE j2.event_id=p.id ORDER BY j2.created_at DESC,j2.id DESC LIMIT 1)
 LEFT JOIN sync_preview v ON v.id=json_extract(j.payload,'$.previewId') `
	statusSQL := `COALESCE(j.status,CASE WHEN a.status='SUCCESS' THEN 'synced' WHEN a.error_type IN ('ANIME_NOT_IN_DB','MAPPING_NOT_FOUND') THEN 'needs_mapping' WHEN a.error_type='MAL_AUTH_FAILED' THEN 'needs_sign_in' WHEN a.id IS NOT NULL OR p.success=0 THEN 'failed' ELSE 'received' END)`
	errorSQL := `CASE WHEN a.id IS NOT NULL THEN COALESCE(a.error_type,'') WHEN p.success=0 THEN COALESCE(NULLIF(p.error_type,''),'UNKNOWN_ERROR') ELSE '' END`
	sourceSQL := `COALESCE(json_extract(i.input,'$.source'),a.source_db,'')`
	where := []string{"1=1"}
	args := []interface{}{}
	if status != "" {
		where = append(where, statusSQL+"=?")
		args = append(args, status)
	}
	if source != "" {
		where = append(where, sourceSQL+"=?")
		args = append(args, source)
	}
	if event != "" {
		where = append(where, "p.event=?")
		args = append(args, event)
	}
	if search != "" {
		where = append(where, `(COALESCE(p.grand_parent_title,'')||' '||COALESCE(p.title,'')||' '||COALESCE(json_extract(v.payload,'$.resolution.title'),json_extract(a.list_details,'$.title'),'')||' '||p.id||' '||COALESCE(json_extract(i.input,'$.sourceId'),a.source_id,0)||' '||COALESCE(json_extract(v.payload,'$.resolution.malId'),a.mal_id,0)) LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escapeLike(search)+"%")
	}
	clause := base + " WHERE " + strings.Join(where, " AND ")
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*)"+clause, args...).Scan(&count); err != nil {
		return nil, 0, err
	}
	query := `SELECT p.id,COALESCE(p.grand_parent_title,''),COALESCE(p.title,''),p.account_title,p.library_section_title,p.event,p.type,COALESCE(p.parent_index,0),COALESCE(p.metadata_index,0),COALESCE(p.rating,0),p.time_stamp,COALESCE(i.input,''),COALESCE(j.payload,''),` + errorSQL + `,COALESCE(json_extract(v.payload,'$.resolution.malId'),a.mal_id,0),COALESCE(json_extract(v.payload,'$.resolution.title'),json_extract(a.list_details,'$.title'),''),` + statusSQL + `,` + sourceSQL + `,COALESCE(json_extract(i.input,'$.sourceId'),a.source_id,0)` + clause + ` ORDER BY p.time_stamp DESC,p.id DESC LIMIT ? OFFSET ?`
	pageArgs := append(append([]interface{}{}, args...), limit, offset)
	rows, err := s.db.QueryContext(ctx, query, pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var e Event
		var inputRaw, jobRaw string
		if err = rows.Scan(&e.ID, &e.Title, &e.EpisodeTitle, &e.Account, &e.Library, &e.Event, &e.MediaType, &e.Season, &e.Episode, &e.Rating, &e.ReceivedAt, &inputRaw, &jobRaw, &e.ErrorCode, &e.MALID, &e.MALTitle, &e.Status, &e.Source, &e.SourceID); err != nil {
			return nil, 0, err
		}
		e.EventID = e.ID
		e.Legacy = inputRaw == ""
		e.LastAction = "Original sync"
		e.ErrorMessage = safeLegacyMessage(e.ErrorCode)
		if e.ErrorCode == "AGENT_NOT_SUPPORTED" || e.ErrorCode == "EXTRACTION_FAILED" {
			e.LastAction = "Plex preparation"
		}
		if inputRaw != "" {
			if err = json.Unmarshal([]byte(inputRaw), &e.Input); err != nil {
				return nil, 0, err
			}
		}
		if e.MediaType == "movie" {
			e.Title = e.EpisodeTitle
			e.Season = 1
			e.Episode = 1
		}
		if jobRaw != "" {
			var j Job
			if err = json.Unmarshal([]byte(jobRaw), &j); err != nil {
				return nil, 0, err
			}
			e.LastAction = j.Kind
			e.ErrorCode = j.Code
			e.ErrorMessage = j.Message
		}
		out = append(out, e)
	}
	return out, count, rows.Err()
}
