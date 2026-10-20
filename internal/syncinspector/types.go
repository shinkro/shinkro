// Package syncinspector owns immutable sync inputs, previews and the single MAL writer.
package syncinspector

import "time"

type Input struct {
	Version      int       `json:"version"`
	EventID      int64     `json:"eventId"`
	Title        string    `json:"title"`
	EpisodeTitle string    `json:"episodeTitle"`
	Account      string    `json:"account"`
	Library      string    `json:"library"`
	ServerID     string    `json:"serverId"`
	Event        string    `json:"event"`
	MediaType    string    `json:"mediaType"`
	Source       string    `json:"source"`
	SourceID     int       `json:"sourceId"`
	Season       int       `json:"season"`
	Episode      int       `json:"episode"`
	Rating       float32   `json:"rating"`
	ReceivedAt   time.Time `json:"receivedAt"`
}
type Event struct {
	Input
	ID           int64  `json:"id"`
	Status       string `json:"status"`
	ErrorCode    string `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
	MALID        int    `json:"malId"`
	MALTitle     string `json:"malTitle"`
	LastAction   string `json:"lastAction"`
	Legacy       bool   `json:"legacy"`
}
type Step struct {
	Label  string `json:"label"`
	Detail string `json:"detail"`
}
type Resolution struct {
	Sources     []SourceSnapshot `json:"sources,omitempty"`
	MALID       int              `json:"malId"`
	Episode     int              `json:"episode"`
	Origin      string           `json:"origin"`
	Title       string           `json:"title"`
	Steps       []Step           `json:"steps"`
	Rule        interface{}      `json:"rule,omitempty"`
	Fingerprint string           `json:"fingerprint"`
}
type MALState struct {
	EvidenceSource  string    `json:"-"`
	MALID           int       `json:"malId"`
	Title           string    `json:"title"`
	PictureURL      string    `json:"pictureUrl"`
	TotalEpisodes   int       `json:"totalEpisodes"`
	Status          string    `json:"status"`
	EpisodesWatched int       `json:"episodesWatched"`
	Score           int       `json:"score"`
	IsRewatching    bool      `json:"isRewatching"`
	RewatchCount    int       `json:"rewatchCount"`
	StartDate       string    `json:"startDate"`
	FinishDate      string    `json:"finishDate"`
	UpdatedAt       time.Time `json:"updatedAt"`
}
type Change struct {
	Field  string      `json:"field"`
	Before interface{} `json:"before"`
	After  interface{} `json:"after"`
}
type Correction struct {
	MALID   int `json:"malId"`
	Episode int `json:"episode"`
}
type Preview struct {
	ActorID       int         `json:"actorId"`
	ID            string      `json:"id"`
	EventID       int64       `json:"eventId"`
	CreatedAt     time.Time   `json:"createdAt"`
	ExpiresAt     time.Time   `json:"expiresAt"`
	Ready         bool        `json:"ready"`
	WritesEnabled bool        `json:"writesEnabled"`
	Code          string      `json:"code"`
	Message       string      `json:"message"`
	Resolution    Resolution  `json:"resolution"`
	Before        *MALState   `json:"before,omitempty"`
	After         *MALState   `json:"after,omitempty"`
	Changes       []Change    `json:"changes"`
	Warnings      []string    `json:"warnings"`
	AccountID     string      `json:"accountId"`
	Correction    *Correction `json:"correction,omitempty"`
}
type Job struct {
	ActorID   int       `json:"actorId"`
	AccountID string    `json:"accountId"`
	ID        string    `json:"id"`
	EventID   int64     `json:"eventId"`
	Kind      string    `json:"kind"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	PreviewID string    `json:"previewId"`
}
type Attempt struct {
	Observed          *MALState  `json:"observed,omitempty"`
	ObservedAt        *time.Time `json:"observedAt,omitempty"`
	ObservationSource string     `json:"observationSource,omitempty"`
	ID                string     `json:"id"`
	JobID             string     `json:"jobId"`
	Kind              string     `json:"kind"`
	Status            string     `json:"status"`
	CreatedAt         time.Time  `json:"createdAt"`
	FinishedAt        time.Time  `json:"finishedAt"`
	Preview           *Preview   `json:"preview,omitempty"`
	Code              string     `json:"code"`
	Message           string     `json:"message"`
}
type Override struct {
	AnyServer          bool      `json:"anyServer"`
	ActorID            int       `json:"actorId"`
	ImpactFingerprint  string    `json:"impactFingerprint,omitempty"`
	ID                 string    `json:"id"`
	Revision           int       `json:"revision"`
	EventID            int64     `json:"eventId"`
	Source             string    `json:"source"`
	SourceID           int       `json:"sourceId"`
	Season             int       `json:"season"`
	Library            string    `json:"library"`
	Account            string    `json:"account"`
	ServerID           string    `json:"serverId"`
	MediaType          string    `json:"mediaType"`
	SourceEpisodeStart int       `json:"sourceEpisodeStart"`
	SourceEpisodeEnd   int       `json:"sourceEpisodeEnd"`
	MALEpisodeStart    int       `json:"malEpisodeStart"`
	MALID              int       `json:"malId"`
	Active             bool      `json:"active"`
	CreatedAt          time.Time `json:"createdAt"`
}
type Detail struct {
	Original       *Event     `json:"original,omitempty"`
	Event          Event      `json:"event"`
	Input          Input      `json:"input"`
	Attempts       []Attempt  `json:"attempts"`
	Legacy         bool       `json:"legacy"`
	ReplayEligible bool       `json:"replayEligible"`
	ReplayReason   string     `json:"replayReason"`
	Overrides      []Override `json:"overrides"`
}
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string        { return e.Message }
func fail(code, message string) error { return &Error{code, message} }
