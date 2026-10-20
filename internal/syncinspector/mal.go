package syncinspector

import (
	"context"
	"github.com/nstratos/go-myanimelist/mal"
	"strconv"
	"time"
)

type MALAdapter struct {
	Client func(context.Context) (*mal.Client, error)
}

func (a MALAdapter) Account(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	c, err := a.Client(ctx)
	if err != nil {
		return "", err
	}
	u, _, err := c.User.MyInfo(ctx)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(u.ID, 10), nil
}
func state(mid int, a *mal.Anime) MALState {
	l := a.MyListStatus
	return MALState{MALID: mid, Title: a.Title, PictureURL: a.MainPicture.Medium, TotalEpisodes: a.NumEpisodes, Status: string(l.Status), EpisodesWatched: l.NumEpisodesWatched, Score: l.Score, IsRewatching: l.IsRewatching, RewatchCount: l.NumTimesRewatched, StartDate: l.StartDate, FinishDate: l.FinishDate, UpdatedAt: l.UpdatedAt}
}
func (a MALAdapter) Read(ctx context.Context, mid int) (MALState, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	c, err := a.Client(ctx)
	if err != nil {
		return MALState{}, err
	}
	v, _, err := c.Anime.Details(ctx, mid, mal.Fields{"num_episodes", "title", "main_picture", "my_list_status{status,score,num_episodes_watched,is_rewatching,updated_at,num_times_rewatched,start_date,finish_date}"})
	if err != nil {
		return MALState{}, err
	}
	return state(mid, v), nil
}
func (a MALAdapter) Write(ctx context.Context, accountID string, mid int, changes []Change) (MALState, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	c, err := a.Client(ctx)
	if err != nil {
		return MALState{}, fail("MAL_AUTH_FAILED", "Reconnect your MyAnimeList account before syncing.")
	}
	u, _, err := c.User.MyInfo(ctx)
	if err != nil {
		return MALState{}, fail("MAL_AUTH_FAILED", "The destination account could not be verified before sending.")
	}
	if strconv.FormatInt(u.ID, 10) != accountID {
		return MALState{}, fail("ACCOUNT_CHANGED", "The MAL account changed before sending.")
	}
	opts := []mal.UpdateMyAnimeListStatusOption{}
	for _, v := range changes {
		switch v.Field {
		case "episodesWatched":
			opts = append(opts, mal.NumEpisodesWatched(asInt(v.After)))
		case "score":
			opts = append(opts, mal.Score(asInt(v.After)))
		case "status":
			opts = append(opts, mal.AnimeStatus(v.After.(string)))
		case "isRewatching":
			opts = append(opts, mal.IsRewatching(v.After.(bool)))
		case "rewatchCount":
			opts = append(opts, mal.NumTimesRewatched(asInt(v.After)))
		case "startDate", "finishDate":
			d, err := time.Parse("2006-01-02", v.After.(string))
			if err != nil {
				return MALState{}, err
			}
			if v.Field == "startDate" {
				opts = append(opts, mal.StartDate(d))
			} else {
				opts = append(opts, mal.FinishDate(d))
			}
		default:
			return MALState{}, fail("INVALID_INTENT", "Unsupported MAL field.")
		}
	}
	if len(opts) == 0 {
		return a.Read(ctx, mid)
	}
	_, response, err := c.Anime.UpdateMyListStatus(ctx, mid, opts...)
	if err != nil {
		if response != nil && response.Response != nil && response.StatusCode >= 400 && response.StatusCode < 500 {
			code := "MAL_API_UPDATE_FAILED"
			message := "MAL rejected the update. Refresh the preview before retrying."
			if response.StatusCode == 401 || response.StatusCode == 403 {
				code = "MAL_AUTH_FAILED"
				message = "Reconnect your MyAnimeList account before syncing."
			}
			if response.StatusCode == 429 {
				code = "MAL_RATE_LIMITED"
				message = "MyAnimeList is rate limiting requests. Wait before previewing and retrying."
			}
			return MALState{}, fail(code, message)
		}
		return MALState{}, err
	}
	// The PATCH response may omit unmodified list fields. Confirm using the
	// same bound client rather than filling absent values from the plan.
	confirmed, _, err := c.Anime.Details(ctx, mid, mal.Fields{"num_episodes", "title", "main_picture", "my_list_status{status,score,num_episodes_watched,is_rewatching,updated_at,num_times_rewatched,start_date,finish_date}"})
	if err != nil {
		return MALState{}, fail("UNCERTAIN_WRITE", "MAL acknowledged the update, but its resulting state could not be read. Preview the event to reconcile before retrying.")
	}
	observed := state(mid, confirmed)
	observed.EvidenceSource = "confirmation_read"
	return observed, nil
}
func asInt(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	}
	return 0
}
