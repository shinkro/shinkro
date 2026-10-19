package notification

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/shinkro/shinkro/internal/domain"
	"github.com/shinkro/shinkro/pkg/sharedhttp"
)

const MAlAnimeURL = "https://myanimelist.net/anime/%d"

type DiscordMessage struct {
	Content interface{}     `json:"content"`
	Embeds  []DiscordEmbeds `json:"embeds,omitempty"`
}

type DiscordEmbeds struct {
	Title       string                `json:"title"`
	URL         string                `json:"url"`
	Image       Image                 `json:"image"`
	Description string                `json:"description"`
	Color       int                   `json:"color"`
	Fields      []DiscordEmbedsFields `json:"fields,omitempty"`
	Timestamp   time.Time             `json:"timestamp"`
}
type DiscordEmbedsFields struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type Image struct {
	URL string `json:"url"`
}

type EmbedColors int

const (
	LIGHT_BLUE EmbedColors = 5814783  // 58b9ff
	RED        EmbedColors = 15548997 // ed4245
	GREEN      EmbedColors = 5763719  // 57f287
	GRAY       EmbedColors = 10070709 // 99aab5
)

type discordSender struct {
	log      zerolog.Logger
	Settings *domain.Notification

	httpClient *http.Client
}

func (a *discordSender) Name() string {
	return "discord"
}

func NewDiscordSender(log zerolog.Logger, settings *domain.Notification) domain.NotificationSender {
	return &discordSender{
		log:      log.With().Str("sender", "discord").Logger(),
		Settings: settings,
		httpClient: &http.Client{
			Timeout:   time.Second * 30,
			Transport: sharedhttp.Transport,
		},
	}
}

func (a *discordSender) Send(event domain.NotificationEvent, payload domain.NotificationPayload) error {
	m := DiscordMessage{
		Content: nil,
		Embeds:  []DiscordEmbeds{a.buildEmbed(event, payload)},
	}

	jsonData, err := json.Marshal(m)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("could not marshal json request for event: %v payload: %v", event, payload))
	}

	req, err := http.NewRequest(http.MethodPost, a.Settings.Webhook, bytes.NewBuffer(jsonData))
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("could not create request for event: %v payload: %v", event, payload))
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", sharedhttp.UserAgent)

	res, err := a.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("client request error for event: %v payload: %v", event, payload))
	}

	defer res.Body.Close()

	a.log.Trace().Msgf("discord response status: %d", res.StatusCode)

	// discord responds with 204, Notifiarr with 204 so lets take all 200 as ok
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		body, err := io.ReadAll(bufio.NewReader(res.Body))
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("could not read body for event: %v payload: %v", event, payload))
		}

		return errors.New(fmt.Sprintf("unexpected status: %v body: %v", res.StatusCode, string(body)))
	}

	a.log.Debug().Msg("notification successfully sent to discord")

	return nil
}

func (a *discordSender) CanSend(event domain.NotificationEvent) bool {
	if a.isEnabled() && a.isEnabledEvent(event) {
		return true
	}
	return false
}

func (a *discordSender) isEnabled() bool {
	if a.Settings.Enabled && a.Settings.Webhook != "" {
		return true
	}
	return false
}

func (a *discordSender) isEnabledEvent(event domain.NotificationEvent) bool {
	for _, e := range a.Settings.Events {
		if e == string(event) {
			return true
		}
		// Backward compatibility: "ERROR" matches both error types
		if e == "ERROR" {
			if event == domain.NotificationEventPlexProcessingError || event == domain.NotificationEventAnimeUpdateError {
				return true
			}
		}
	}

	return false
}

func (a *discordSender) buildEmbed(event domain.NotificationEvent, payload domain.NotificationPayload) DiscordEmbeds {

	color := LIGHT_BLUE
	switch event {
	case domain.NotificationEventSuccess:
		color = GREEN
	case domain.NotificationEventPlexProcessingError, domain.NotificationEventAnimeUpdateError:
		color = RED
	case domain.NotificationEventTest:
		color = LIGHT_BLUE
	}

	var fields []DiscordEmbedsFields

	if payload.PlexEvent != "" {
		f := DiscordEmbedsFields{
			Name:   "New Plex Event",
			Value:  string(payload.PlexEvent),
			Inline: true,
		}
		fields = append(fields, f)
	}

	if payload.PlexSource != "" {
		f := DiscordEmbedsFields{
			Name:   "Plex Source",
			Value:  string(payload.PlexSource),
			Inline: true,
		}
		fields = append(fields, f)
	}

	if payload.AnimeLibrary != "" {
		f := DiscordEmbedsFields{
			Name:   "Anime Library",
			Value:  payload.AnimeLibrary,
			Inline: true,
		}
		fields = append(fields, f)
	}

	if payload.AnimeStatus != "" {
		f := DiscordEmbedsFields{
			Name:   "Watch Status",
			Value:  payload.AnimeStatus,
			Inline: true,
		}
		fields = append(fields, f)
	}

	if payload.EpisodesWatched > 0 {
		totalEps := strconv.Itoa(payload.EpisodesTotal)
		if payload.EpisodesTotal == 0 {
			totalEps = "?"
		}
		f := DiscordEmbedsFields{
			Name:   "Episodes Watched",
			Value:  fmt.Sprintf("%d/%s", payload.EpisodesWatched, totalEps),
			Inline: true,
		}
		fields = append(fields, f)
	}

	if payload.Score > 0 {
		score := strconv.Itoa(payload.Score)
		if payload.Score == 0 {
			score = "Not Scored"
		}
		f := DiscordEmbedsFields{
			Name:   "Score",
			Value:  score,
			Inline: true,
		}
		fields = append(fields, f)
	}

	if payload.TimesRewatched > 0 {
		f := DiscordEmbedsFields{
			Name:   "Times Rewatched",
			Value:  strconv.Itoa(payload.TimesRewatched),
			Inline: true,
		}
		fields = append(fields, f)
	}

	if payload.StartDate != "" {
		f := DiscordEmbedsFields{
			Name:   "Start Date",
			Value:  payload.StartDate,
			Inline: true,
		}
		fields = append(fields, f)
	}

	if payload.FinishDate != "" {
		f := DiscordEmbedsFields{
			Name:   "Finish Date",
			Value:  payload.FinishDate,
			Inline: true,
		}
		fields = append(fields, f)
	}

	embed := DiscordEmbeds{
		Title:       string(event),
		Description: "New shinkro Update!",
		Color:       int(color),
		Fields:      fields,
		Timestamp:   time.Now(),
	}

	if payload.Subject != "" && payload.Message != "" {
		embed.Title = payload.Subject
		embed.Description = payload.Message
	}

	if payload.MALID > 0 && payload.MediaName != "" {
		embed.URL = fmt.Sprintf(MAlAnimeURL, payload.MALID)
		embed.Title = payload.MediaName
	}

	if payload.PictureURL != "" {
		embed.Image = Image{
			URL: payload.PictureURL,
		}
	}

	if len(payload.UpdatedServices) > 0 {
		fields = append(fields, DiscordEmbedsFields{
			Name:   "Updated Services",
			Value:  strings.Join(payload.UpdatedServices, " + "),
			Inline: true,
		})
		embed.Fields = fields
		embed.Description = fmt.Sprintf("Synced to %s", strings.Join(payload.UpdatedServices, " & "))
	}

	return embed
}
