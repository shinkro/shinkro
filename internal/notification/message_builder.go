package notification

import (
	"fmt"
	"github.com/dustin/go-humanize"
	"html"
	"strings"

	"github.com/shinkro/shinkro/internal/domain"
)

type MessageBuilder interface {
	BuildBody(payload domain.NotificationPayload) string
}

type ConditionMessagePart struct {
	Condition bool
	Format    string
	Bits      []interface{}
}

// MessageBuilderPlainText constructs the body of the notification message in plain text format.
type MessageBuilderPlainText struct{}

// BuildBody constructs the body of the notification message.
func (b *MessageBuilderPlainText) BuildBody(payload domain.NotificationPayload) string {
	messageParts := []ConditionMessagePart{
		{payload.Sender != "", "%v\n", []interface{}{payload.Sender}},
		{payload.Subject != "" && payload.Message != "", "%v\n%v", []interface{}{payload.Subject, payload.Message}},
		{payload.PlexEvent != "", "New Plex Event: %v\n", []interface{}{payload.PlexEvent}},
		{payload.PlexSource != "", "Plex Source: %v\n", []interface{}{payload.PlexSource}},
		{payload.MediaName != "", "Show: %v\n", []interface{}{payload.MediaName}},
		{payload.AnimeLibrary != "", "Anime Library: %v\n", []interface{}{payload.AnimeLibrary}},
		{payload.EpisodesWatched > 0, "Episodes Watched: %v\n", []interface{}{payload.EpisodesWatched}},
		{payload.AnimeStatus != "", "MAL Watch Status: %v\n", []interface{}{payload.AnimeStatus}},
		{payload.Score > 0, "Score: %v\n", []interface{}{payload.Score}},
		{payload.TimesRewatched > 0, "Times Rewatched: %v\n", []interface{}{payload.TimesRewatched}},
		{payload.StartDate != "", "Start Date: %v\n", []interface{}{payload.StartDate}},
		{payload.FinishDate != "", "Finish Date: %v\n", []interface{}{payload.FinishDate}},
	}

	return formatMessageContent(messageParts)
}

// MessageBuilderHTML constructs the body of the notification message in HTML format.
type MessageBuilderHTML struct{}

func (b *MessageBuilderHTML) BuildBody(payload domain.NotificationPayload) string {
	messageParts := []ConditionMessagePart{
		{payload.Sender != "", "<b>%v</b>\n", []interface{}{html.EscapeString(payload.Sender)}},
		{payload.Subject != "" && payload.Message != "", "<b>%v</b> %v\n", []interface{}{html.EscapeString(payload.Subject), html.EscapeString(payload.Message)}},
		{payload.PlexEvent != "", "<b>New Plex Event:</b> %v\n", []interface{}{html.EscapeString(string(payload.PlexEvent))}},
		{payload.PlexSource != "", "<b>Plex Source:</b> %v\n", []interface{}{html.EscapeString(string(payload.PlexSource))}},
		{payload.MediaName != "", "<b>Show:</b> %v\n", []interface{}{html.EscapeString(payload.MediaName)}},
		{payload.AnimeLibrary != "", "<b>Anime Library:</b> %v\n", []interface{}{html.EscapeString(payload.AnimeLibrary)}},
		{payload.EpisodesWatched > 0, "<b>Episodes Watched:</b> %v\n", []interface{}{humanize.Comma(int64(payload.EpisodesWatched))}},
		{payload.AnimeStatus != "", "<b>MAL Watch Status:</b> %v\n", []interface{}{html.EscapeString(payload.AnimeStatus)}},
		{payload.Score > 0, "<b>Score:</b> %v\n", []interface{}{humanize.Comma(int64(payload.Score))}},
		{payload.TimesRewatched > 0, "<b>Times Rewatched:</b> %v\n", []interface{}{humanize.Comma(int64(payload.TimesRewatched))}},
		{payload.StartDate != "", "<b>Start Date:</b> %v\n", []interface{}{html.EscapeString(payload.StartDate)}},
		{payload.FinishDate != "", "<b>Finish Date:</b> %v\n", []interface{}{html.EscapeString(payload.FinishDate)}},
	}

	return formatMessageContent(messageParts)
}

func formatMessageContent(messageParts []ConditionMessagePart) string {
	var builder strings.Builder
	for _, part := range messageParts {
		if part.Condition {
			builder.WriteString(fmt.Sprintf(part.Format, part.Bits...))
		}
	}
	return builder.String()
}

// BuildTitle constructs the title of the notification message.
func BuildTitle(event domain.NotificationEvent) string {
	titles := map[domain.NotificationEvent]string{
		domain.NotificationEventAppUpdateAvailable: "shinkro Update Available",
		domain.NotificationEventSuccess:            "MAL Update Successful",
		domain.NotificationEventPlexProcessingError: "Plex Processing Error",
		domain.NotificationEventAnimeUpdateError:    "Anime Update Error",
		domain.NotificationEventTest:               "TEST",
	}

	if title, ok := titles[event]; ok {
		return title
	}

	return "NEW EVENT"
}
