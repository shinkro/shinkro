package syncinspector

import (
	"context"
	"time"
)

// Unapplied previews expire after five minutes and need not accumulate forever.
// Every job-referenced intent is retained, including uncertain writes which may
// still need reconciliation. Event/attempt retention follows explicit deletion.
func (s *Service) prunePreviews(ctx context.Context, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sync_preview WHERE created_at<? AND id NOT IN (SELECT json_extract(payload,'$.previewId') FROM sync_job WHERE COALESCE(json_extract(payload,'$.previewId'),'')<>'')`, now.Add(-24*time.Hour))
	return err
}
