package domain

import "time"

// Event names
const (
	EventPlexProcessedSuccess = "plex:processed:success"
	EventPlexProcessedError   = "plex:processed:error"
	EventNotificationSend     = "notification:send"
	EventAnimeUpdateSuccess   = "animeupdate:success"
	EventAnimeUpdateFailed    = "animeupdate:failed"
	EventAnilistSyncFailed    = "anilist:sync:failed"
)

// PlexProcessedSuccessEvent is published when a Plex payload is successfully processed (extraction complete, before MAL update)
type PlexProcessedSuccessEvent struct {
	PlexID     int64
	Plex       *Plex
	AnimeUpdate *AnimeUpdate // Contains extracted anime info needed for MAL update
	Timestamp  time.Time
}

// PlexProcessedErrorEvent is published when a Plex payload processing fails
type PlexProcessedErrorEvent struct {
	PlexID       int64
	Plex         *Plex
	ErrorType    PlexErrorType
	ErrorMessage string
	Timestamp    time.Time
}

// NotificationSendEvent is published when a notification should be sent
type NotificationSendEvent struct {
	Event   NotificationEvent
	Payload NotificationPayload
}

// AnimeUpdateSuccessEvent is published when MAL update succeeds
type AnimeUpdateSuccessEvent struct {
	PlexID        int64
	AnimeUpdate   *AnimeUpdate
	AnilistSynced bool
	AnilistID     int
	Timestamp     time.Time
}

type AnilistSyncFailedEvent struct {
	AnimeUpdate  *AnimeUpdate
	AnilistID    int
	ErrorMessage string
	Timestamp    time.Time
}

// AnimeUpdateFailedEvent is published when MAL update fails
type AnimeUpdateFailedEvent struct {
	AnimeUpdate  *AnimeUpdate
	ErrorType    AnimeUpdateErrorType
	ErrorMessage string
	Timestamp    time.Time
}
