package testdata

import (
	"strconv"
	"time"

	"github.com/nstratos/go-myanimelist/mal"
	"github.com/shinkro/shinkro/internal/domain"
)

// Plex Webhook Payloads

// RawPlexWebhookHAMAEpisode returns a raw JSON string matching real Plex webhook format
// Based on actual payload structure from production
func RawPlexWebhookHAMAEpisode() string {
	return `{"Account":{"id":12345,"thumb":"https://plex.tv/users/testuser/avatar?c=1234567890","title":"TestUser"},"Metadata":{"Image":[{"alt":"HDTV- - Please","type":"coverPoster","url":"/library/metadata/3484/thumb/1758472311"}],"addedAt":1766938180,"art":"/library/metadata/3484/art/1758472311","contentRating":"TV-Y7","grandparentArt":"/library/metadata/3484/art/1758472311","grandparentGuid":"com.plexapp.agents.hama://tvdb-81797?lang=en","grandparentKey":"/library/metadata/3484","grandparentRatingKey":"3484","grandparentThumb":"/library/metadata/3484/thumb/1758472311","grandparentTitle":"One Piece","guid":"com.plexapp.agents.hama://tvdb-81797/22/70?lang=en","index":70,"key":"/library/metadata/31444","lastViewedAt":1767117106,"librarySectionID":1,"librarySectionKey":"/library/sections/1","librarySectionTitle":"Anime","librarySectionType":"show","parentGuid":"com.plexapp.agents.hama://tvdb-81797/22?lang=en","parentIndex":22,"parentKey":"/library/metadata/29456","parentRatingKey":"29456","parentThumb":"/library/metadata/29456/thumb/1758206535","parentTitle":"Season 22","ratingKey":"31444","summary":"","thumb":"/library/metadata/31444/thumb/1766938222","title":"HDTV- - Please","type":"episode","updatedAt":1766938222,"viewCount":1},"Player":{"local":false,"publicAddress":"192.168.1.100","title":"Chrome","uuid":"test-player-uuid"},"Server":{"title":"Test Server","uuid":"test-server-uuid-12345"},"event":"media.scrobble","owner":true,"user":true}`
}

// RawPlexWebhookHAMAAniDBEpisode returns HAMA agent with AniDB ID
func RawPlexWebhookHAMAAniDBEpisode() string {
	return `{"Account":{"id":1,"thumb":"","title":"TestUser"},"Metadata":{"grandparentTitle":"Attack on Titan","guid":"com.plexapp.agents.hama://anidb-12345/1/5?lang=en","index":5,"parentIndex":1,"librarySectionTitle":"Anime","type":"episode","title":"Episode 5"},"Player":{"local":true,"title":"Chrome","uuid":"test-uuid"},"Server":{"title":"Test Server","uuid":"test-server-uuid"},"event":"media.scrobble","owner":true,"user":true}`
}

// RawPlexWebhookMALAgentEpisode returns MAL Agent format
func RawPlexWebhookMALAgentEpisode() string {
	return `{"Account":{"id":1,"thumb":"","title":"TestUser"},"Metadata":{"grandparentTitle":"Code Geass","guid":"net.fribbtastic.coding.plex.myanimelist://1575?lang=en","index":10,"parentIndex":1,"librarySectionTitle":"Anime","type":"episode","title":"Episode 10"},"Player":{"local":true,"title":"Chrome","uuid":"test-uuid"},"Server":{"title":"Test Server","uuid":"test-server-uuid"},"event":"media.scrobble","owner":true,"user":true}`
}

// RawPlexWebhookPlexAgentEpisode returns Plex Agent with GUID array
func RawPlexWebhookPlexAgentEpisode() string {
	return `{"Account":{"id":1,"thumb":"","title":"TestUser"},"Metadata":{"grandparentTitle":"Demon Slayer","guid":[{"id":"tvdb://362753"},{"id":"tmdb://12345"}],"index":12,"parentIndex":2,"librarySectionTitle":"Anime","type":"episode","title":"Episode 12"},"Player":{"local":true,"title":"Chrome","uuid":"test-uuid"},"Server":{"title":"Test Server","uuid":"test-server-uuid"},"event":"media.scrobble","owner":true,"user":true}`
}

// RawPlexWebhookMovie returns a movie webhook payload
func RawPlexWebhookMovie() string {
	return `{"Account":{"id":1,"thumb":"","title":"TestUser"},"Metadata":{"title":"Your Name","guid":[{"id":"tmdb://372058"}],"librarySectionTitle":"Anime Movies","type":"movie","ratingKey":"12345"},"Player":{"local":true,"title":"Chrome","uuid":"test-uuid"},"Server":{"title":"Test Server","uuid":"test-server-uuid"},"event":"media.rate","rating":9.0,"owner":true,"user":true}`
}

// RawPlexWebhookRateEvent returns a rating event
func RawPlexWebhookRateEvent() string {
	return `{"Account":{"id":1,"thumb":"","title":"TestUser"},"Metadata":{"grandparentTitle":"Attack on Titan","guid":"com.plexapp.agents.hama://anidb-12345/1/5?lang=en","index":5,"parentIndex":1,"librarySectionTitle":"Anime","type":"episode","title":"Episode 5"},"Player":{"local":true,"title":"Chrome","uuid":"test-uuid"},"Server":{"title":"Test Server","uuid":"test-server-uuid"},"event":"media.rate","rating":8.5,"owner":true,"user":true}`
}

// RawPlexWebhookEmptyPayload returns minimal valid payload
func RawPlexWebhookEmptyPayload() string {
	return `{"event":"media.scrobble"}`
}

// RawPlexWebhookInvalidAgent returns unsupported agent
func RawPlexWebhookInvalidAgent() string {
	return `{"Account":{"id":1,"title":"TestUser"},"Metadata":{"grandparentTitle":"Test","guid":"com.plexapp.agents.thetvdb://12345","index":1,"parentIndex":1,"librarySectionTitle":"Anime","type":"episode"},"event":"media.scrobble","owner":true,"user":true}`
}

// Tautulli Payloads

// RawTautulliEpisode returns Tautulli format episode payload
func RawTautulliEpisode() string {
	return `{"Account":{"title":"TestUser"},"event":"media.scrobble","Metadata":{"title":"Episode 5","type":"episode","parentIndex":"1","index":"5","guid":"com.plexapp.agents.hama://anidb-12345/1/1?lang=en","grandparentKey":"/library/metadata/123","grandparentTitle":"Attack on Titan","librarySectionTitle":"Anime"}}`
}

// RawTautulliMovie returns Tautulli format movie payload
func RawTautulliMovie() string {
	return `{"Account":{"title":"TestUser"},"event":"media.rate","Metadata":{"title":"Your Name","type":"movie","parentIndex":"1","index":"1","guid":[{"id":"tmdb://372058"}],"grandparentKey":"","grandparentTitle":"","librarySectionTitle":"Anime Movies"}}`
}

// RawTautulliPlexAgent returns Tautulli with Plex Agent GUID array
func RawTautulliPlexAgent() string {
	return `{"Account":{"title":"TestUser"},"event":"media.scrobble","Metadata":{"title":"Episode 12","type":"episode","parentIndex":"2","index":"12","guid":[{"id":"tvdb://362753"},{"id":"tmdb://12345"}],"grandparentKey":"/library/metadata/456","grandparentTitle":"Demon Slayer","librarySectionTitle":"Anime"}}`
}

// Domain Object Mocks

// NewMockPlex creates a Plex domain object
func NewMockPlex() *domain.Plex {
	return &domain.Plex{
		ID:        12345,
		Rating:    8.5,
		TimeStamp: time.Now(),
		Event:     domain.PlexScrobbleEvent,
		User:      true,
		Source:    domain.PlexWebhook,
		Owner:     true,
		Account: struct {
			Id           int    `json:"id"`
			ThumbnailUrl string `json:"thumb"`
			Title        string `json:"title"`
		}{
			Id:           15637740,
			ThumbnailUrl: "https://plex.tv/users/635274dc88465e65/avatar?c=1767086671",
			Title:        "TestUser",
		},
		Server: struct {
			Title string `json:"title"`
			UUID  string `json:"uuid"`
		}{
			Title: "Test Server",
			UUID:  "test-server-uuid",
		},
		Player: struct {
			Local         bool   `json:"local"`
			PublicAddress string `json:"publicAddress"`
			Title         string `json:"title"`
			UUID          string `json:"uuid"`
		}{
			Local:         true,
			PublicAddress: "",
			Title:         "Chrome",
			UUID:          "test-uuid",
		},
		Metadata: domain.Metadata{
			Type:                domain.PlexEpisode,
			Title:               "Episode 5",
			GrandparentTitle:    "Attack on Titan",
			Index:               5,
			ParentIndex:         1,
			LibrarySectionTitle: "Anime",
			GUID: domain.GUID{
				GUID: "com.plexapp.agents.hama://anidb-12345/1/5?lang=en",
			},
		},
	}
}

// NewMockPlexWithHAMA creates Plex with HAMA agent
func NewMockPlexWithHAMA(db string, id int) *domain.Plex {
	p := NewMockPlex()
	p.Metadata.GUID = domain.GUID{
		GUID: "com.plexapp.agents.hama://" + db + "-" + strconv.Itoa(id) + "/1/5?lang=en",
	}
	return p
}

// NewMockPlexWithMALAgent creates Plex with MAL Agent
func NewMockPlexWithMALAgent(malid int) *domain.Plex {
	p := NewMockPlex()
	p.Metadata.GUID = domain.GUID{
		GUID: "net.fribbtastic.coding.plex.myanimelist://" + strconv.Itoa(malid) + "?lang=en",
	}
	return p
}

// NewMockPlexWithPlexAgent creates Plex with Plex Agent (GUID array)
func NewMockPlexWithPlexAgent(mediaType domain.PlexMediaType) *domain.Plex {
	p := NewMockPlex()
	p.Metadata.Type = mediaType
	if mediaType == domain.PlexEpisode {
		p.Metadata.GUID = domain.GUID{
			GUIDS: []struct {
				ID string `json:"id"`
			}{
				{ID: "tvdb://362753"},
				{ID: "tmdb://12345"},
			},
		}
	} else {
		p.Metadata.GUID = domain.GUID{
			GUIDS: []struct {
				ID string `json:"id"`
			}{
				{ID: "tmdb://372058"},
			},
		}
	}
	return p
}

// NewMockPlexMovie creates a movie Plex object
func NewMockPlexMovie() *domain.Plex {
	p := NewMockPlex()
	p.Metadata.Type = domain.PlexMovie
	p.Metadata.Title = "Your Name"
	p.Metadata.Index = 1
	p.Metadata.ParentIndex = 1
	p.Metadata.LibrarySectionTitle = "Anime Movies"
	p.Metadata.GrandparentTitle = ""
	p.Event = domain.PlexRateEvent
	p.Rating = 9.0
	p.Metadata.GUID = domain.GUID{
		GUIDS: []struct {
			ID string `json:"id"`
		}{
			{ID: "tmdb://372058"},
		},
	}
	return p
}

// NewMockAnimeUpdate creates an AnimeUpdate domain object
func NewMockAnimeUpdate() *domain.AnimeUpdate {
	return &domain.AnimeUpdate{
		ID:         1,
		MALId:      1575,
		SourceDB:   domain.TVDB,
		SourceId:   362753,
		EpisodeNum: 5,
		SeasonNum:  1,
		Timestamp:  time.Now(),
		PlexId:     12345,
		Plex:       NewMockPlex(),
		ListDetails: domain.ListDetails{
			Status:          mal.AnimeStatusWatching,
			RewatchNum:      0,
			TotalEpisodeNum: 25,
			WatchedNum:      4,
			Title:           "Attack on Titan",
			PictureURL:      "https://cdn.myanimelist.net/images/anime/10/47347.jpg",
		},
		Status: domain.AnimeUpdateStatusSuccess,
	}
}

// NewMockAnimeUpdateWithStatus creates AnimeUpdate with specific status
func NewMockAnimeUpdateWithStatus(status domain.AnimeUpdateStatusType, errorType domain.AnimeUpdateErrorType) *domain.AnimeUpdate {
	au := NewMockAnimeUpdate()
	au.Status = status
	au.ErrorType = errorType
	if status == domain.AnimeUpdateStatusFailed {
		au.ErrorMessage = getErrorMessageForErrorType(errorType)
	}
	return au
}

// NewMockAnimeUpdateFirstEpisode creates AnimeUpdate for first episode
func NewMockAnimeUpdateFirstEpisode() *domain.AnimeUpdate {
	au := NewMockAnimeUpdate()
	au.EpisodeNum = 1
	au.ListDetails.WatchedNum = 0
	au.ListDetails.Status = mal.AnimeStatusPlanToWatch
	return au
}

// NewMockAnimeUpdateLastEpisode creates AnimeUpdate for last episode (completion)
func NewMockAnimeUpdateLastEpisode() *domain.AnimeUpdate {
	au := NewMockAnimeUpdate()
	au.EpisodeNum = 25
	au.ListDetails.WatchedNum = 24
	au.ListDetails.TotalEpisodeNum = 25
	au.ListDetails.Status = mal.AnimeStatusWatching
	return au
}

// NewMockAnimeUpdateRewatching creates AnimeUpdate for rewatching scenario
func NewMockAnimeUpdateRewatching() *domain.AnimeUpdate {
	au := NewMockAnimeUpdate()
	au.ListDetails.Status = mal.AnimeStatusCompleted
	au.ListDetails.RewatchNum = 1
	au.ListDetails.WatchedNum = 25
	au.EpisodeNum = 5 // Watching episode 5 again
	return au
}

// NewMockListDetails creates ListDetails
func NewMockListDetails() domain.ListDetails {
	return domain.ListDetails{
		Status:          mal.AnimeStatusWatching,
		RewatchNum:      0,
		TotalEpisodeNum: 25,
		WatchedNum:      5,
		Title:           "Attack on Titan",
		PictureURL:      "https://cdn.myanimelist.net/images/anime/10/47347.jpg",
	}
}

// NewMockListDetailsWithStatus creates ListDetails with specific status
func NewMockListDetailsWithStatus(status mal.AnimeStatus, watched, total int) domain.ListDetails {
	return domain.ListDetails{
		Status:          status,
		RewatchNum:      0,
		TotalEpisodeNum: total,
		WatchedNum:      watched,
		Title:           "Test Anime",
		PictureURL:      "",
	}
}

// NewMockMALAnimeListStatus creates MAL AnimeListStatus
func NewMockMALAnimeListStatus() mal.AnimeListStatus {
	return mal.AnimeListStatus{
		Status:             mal.AnimeStatusWatching,
		Score:              9,
		NumEpisodesWatched: 5,
		IsRewatching:       false,
		UpdatedAt:          time.Now(),
		StartDate:          "2024-01-01",
		FinishDate:         "",
	}
}

// NewMockMALAnimeListStatusCompleted creates completed status
func NewMockMALAnimeListStatusCompleted() mal.AnimeListStatus {
	return mal.AnimeListStatus{
		Status:             mal.AnimeStatusCompleted,
		Score:              10,
		NumEpisodesWatched: 25,
		IsRewatching:       false,
		UpdatedAt:          time.Now(),
		StartDate:          "2024-01-01",
		FinishDate:         "2024-01-25",
	}
}

// Mapping Mocks

// NewMockAnimeTV creates AnimeTV mapping
func NewMockAnimeTV() domain.AnimeTV {
	return domain.AnimeTV{
		Malid:      1575,
		Title:      "Attack on Titan",
		Type:       "TV",
		Tvdbid:     362753,
		TvdbSeason: 1,
		Start:      0,
		UseMapping: false,
	}
}

// NewMockAnimeTVWithMapping creates AnimeTV with complex mapping
func NewMockAnimeTVWithMapping() domain.AnimeTV {
	return domain.AnimeTV{
		Malid:      21,
		Title:      "One Piece",
		Type:       "TV",
		Tvdbid:     81797,
		TvdbSeason: 0,
		Start:      0,
		UseMapping: true,
		AnimeMapping: []domain.AnimeMapping{
			{TvdbSeason: 10, Start: 196},
			{TvdbSeason: 12, Start: 326},
			{TvdbSeason: 15, Start: 517},
			{TvdbSeason: 21, Start: 892},
		},
	}
}

// NewMockAnimeTVWithExplicitMapping creates AnimeTV with explicit episode mapping
func NewMockAnimeTVWithExplicitMapping() domain.AnimeTV {
	return domain.AnimeTV{
		Malid:      17074,
		Title:      "Monogatari Series",
		Type:       "TV",
		Tvdbid:     102261,
		TvdbSeason: 0,
		Start:      0,
		UseMapping: true,
		AnimeMapping: []domain.AnimeMapping{
			{
				TvdbSeason:       0,
				Start:            0,
				MappingType:      "explicit",
				ExplicitEpisodes: map[int]int{7: 6, 8: 11, 9: 16},
			},
			{
				TvdbSeason:      3,
				Start:           1,
				MappingType:     "range",
				SkipMalEpisodes: []int{6, 11, 16},
			},
		},
	}
}

// NewMockAnimeMovie creates AnimeMovie mapping
func NewMockAnimeMovie() domain.AnimeMovie {
	return domain.AnimeMovie{
		MainTitle: "Your Name",
		TMDBID:    372058,
		MALID:     32281,
	}
}

// NewMockAnimeMapDetails creates AnimeMapDetails
func NewMockAnimeMapDetails() *domain.AnimeMapDetails {
	return &domain.AnimeMapDetails{
		Malid:       1575,
		Start:       0,
		UseMapping:  false,
		MappingType: "range",
	}
}

// NewMockAnimeMapDetailsWithExplicit creates AnimeMapDetails with explicit mapping
func NewMockAnimeMapDetailsWithExplicit() *domain.AnimeMapDetails {
	return &domain.AnimeMapDetails{
		Malid:            17074,
		Start:            0,
		UseMapping:       true,
		MappingType:      "explicit",
		ExplicitEpisodes: map[int]int{7: 6, 8: 11, 9: 16},
	}
}

// NewMockAnimeMapDetailsWithSkips creates AnimeMapDetails with skip logic
func NewMockAnimeMapDetailsWithSkips() *domain.AnimeMapDetails {
	return &domain.AnimeMapDetails{
		Malid:           17074,
		Start:           1,
		UseMapping:      true,
		MappingType:     "range",
		SkipMalEpisodes: []int{6, 11, 16},
	}
}

// PlexSettings Mocks

// NewMockPlexSettings creates PlexSettings
func NewMockPlexSettings() *domain.PlexSettings {
	return &domain.PlexSettings{
		Host:              "192.168.1.100",
		Port:              32400,
		TLS:               false,
		TLSSkip:           false,
		AnimeLibraries:    []string{"Anime", "Anime Movies"},
		PlexUser:          "TestUser",
		PlexClientEnabled: true,
		ClientID:          "shinkro-abc123",
		Token:             []byte("test-token"),
		TokenIV:           []byte("test-iv"),
	}
}

// Notification Mocks

// NewMockNotificationPayload creates NotificationPayload
func NewMockNotificationPayload() domain.NotificationPayload {
	return domain.NotificationPayload{
		Subject:         "MAL Update Successful",
		Message:         "",
		Event:           domain.NotificationEventSuccess,
		MediaName:       "Attack on Titan",
		MALID:           1575,
		AnimeLibrary:    "Anime",
		EpisodesWatched: 5,
		EpisodesTotal:   25,
		TimesRewatched:  0,
		PictureURL:      "https://cdn.myanimelist.net/images/anime/10/47347.jpg",
		StartDate:       "2024-01-01",
		FinishDate:      "",
		AnimeStatus:     "Watching",
		Score:           9,
		PlexEvent:       domain.PlexScrobbleEvent,
		PlexSource:      domain.PlexWebhook,
		Timestamp:       time.Now(),
		Sender:          "shinkro",
	}
}

// NewMockNotificationPayloadError creates error notification payload
func NewMockNotificationPayloadError(event domain.NotificationEvent) domain.NotificationPayload {
	payload := NewMockNotificationPayload()
	payload.Event = event
	switch event {
	case domain.NotificationEventPlexProcessingError:
		payload.Subject = "Unsupported Metadata Agent"
		payload.Message = "Failed to process Plex payload for: Attack on Titan\n\nError: metadata agent not supported"
	case domain.NotificationEventAnimeUpdateError:
		payload.Subject = "Mapping Not Found"
		payload.Message = "Failed to update MyAnimeList for: Attack on Titan\n\nError: mapping not found"
	}
	return payload
}

// User Mocks

// NewMockUser creates User
func NewMockUser() *domain.User {
	return &domain.User{
		ID:       1,
		Username: "testuser",
		Password: "$argon2id$v=19$m=65536,t=3,p=4$hashed", // Example hash format
	}
}

// NewMockCreateUserRequest creates CreateUserRequest
func NewMockCreateUserRequest() domain.CreateUserRequest {
	return domain.CreateUserRequest{
		Username: "newuser",
		Password: "password123",
	}
}

// NewMockUpdateUserRequest creates UpdateUserRequest
func NewMockUpdateUserRequest() domain.UpdateUserRequest {
	return domain.UpdateUserRequest{
		UsernameCurrent: "testuser",
		UsernameNew:     "updateduser",
		PasswordNewHash: "$argon2id$v=19$m=65536,t=3,p=4$newhashed",
	}
}

// Helper Functions

func getErrorMessageForErrorType(errorType domain.AnimeUpdateErrorType) string {
	messages := map[domain.AnimeUpdateErrorType]string{
		domain.AnimeUpdateErrorMALAuthFailed:      "token expired",
		domain.AnimeUpdateErrorMappingNotFound:    "anime not found in map",
		domain.AnimeUpdateErrorAnimeNotInDB:       "anime not in database",
		domain.AnimeUpdateErrorMALAPIFetchFailed:  "rate limit exceeded",
		domain.AnimeUpdateErrorMALAPIUpdateFailed: "update request failed",
		domain.AnimeUpdateErrorUnknown:            "unknown error occurred",
	}
	if msg, ok := messages[errorType]; ok {
		return msg
	}
	return "error occurred"
}
