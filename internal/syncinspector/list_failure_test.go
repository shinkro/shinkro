package syncinspector

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListIncludesPlexPreparationFailuresWithoutMALHistory(t *testing.T) {
	s, db, _, _, ctx := fixture(t, false)
	for n := int64(1); n <= 3; n++ {
		seed(t, s, direct(n, int(n)))
	}
	_, err := db.Exec(`UPDATE plex_payload SET success=0,error_type='EXTRACTION_FAILED',error_msg='secret-token-and-private-path' WHERE id=1`)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE plex_payload SET success=0,error_type='AGENT_NOT_SUPPORTED' WHERE id=2`)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE plex_payload SET success=1 WHERE id=3`)
	require.NoError(t, err)

	failed, count, err := s.List(ctx, 25, 0, "", "failed", "", "")
	require.NoError(t, err)
	require.Equal(t, 2, count)
	require.Len(t, failed, 2)
	byID := map[int64]Event{}
	for _, event := range failed {
		byID[event.ID] = event
		require.Equal(t, "failed", event.Status)
		require.Zero(t, event.MALID)
	}
	require.Equal(t, "EXTRACTION_FAILED", byID[1].ErrorCode)
	require.Equal(t, "AGENT_NOT_SUPPORTED", byID[2].ErrorCode)
	encoded, err := json.Marshal(failed)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "secret-token-and-private-path")

	received, count, err := s.List(ctx, 25, 0, "", "received", "", "")
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.Len(t, received, 1)
	require.EqualValues(t, 3, received[0].ID)
}
