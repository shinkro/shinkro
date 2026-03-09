package notification

import (
	"strings"
	"testing"

	"github.com/shinkro/shinkro/internal/domain"
	"github.com/shinkro/shinkro/internal/testdata"

	"github.com/stretchr/testify/assert"
)

func TestMessageBuilderPlainText_BuildBody(t *testing.T) {
	tests := []struct {
		name     string
		payload  domain.NotificationPayload
		validate func(*testing.T, string)
	}{
		{
			name:    "full payload",
			payload: testdata.NewMockNotificationPayload(),
			validate: func(t *testing.T, body string) {
				// Subject only appears when both Subject AND Message are present
				// Mock has Subject but empty Message, so Subject won't appear
				assert.Contains(t, body, "Attack on Titan")
				assert.Contains(t, body, "Anime")
				assert.Contains(t, body, "Episodes Watched: 5")
				assert.Contains(t, body, "MAL Watch Status: Watching")
				assert.Contains(t, body, "Score: 9")
			},
		},
		{
			name: "minimal payload with PlexEvent",
			payload: domain.NotificationPayload{
				Event:     domain.NotificationEventSuccess,
				PlexEvent: domain.PlexScrobbleEvent,
			},
			validate: func(t *testing.T, body string) {
				// Should contain event-related content
				assert.Contains(t, body, "New Plex Event:")
				assert.NotEmpty(t, body)
			},
		},
		{
			name: "payload with subject and message",
			payload: domain.NotificationPayload{
				Subject: "Test Subject",
				Message: "Test Message",
				Event:   domain.NotificationEventSuccess,
			},
			validate: func(t *testing.T, body string) {
				assert.Contains(t, body, "Test Subject")
				assert.Contains(t, body, "Test Message")
			},
		},
		{
			name: "payload with rewatch",
			payload: domain.NotificationPayload{
				Event:          domain.NotificationEventSuccess,
				TimesRewatched: 3,
			},
			validate: func(t *testing.T, body string) {
				assert.Contains(t, body, "Times Rewatched: 3")
			},
		},
		{
			name: "payload with dates",
			payload: domain.NotificationPayload{
				Event:      domain.NotificationEventSuccess,
				StartDate:  "2024-01-01",
				FinishDate: "2024-01-25",
			},
			validate: func(t *testing.T, body string) {
				assert.Contains(t, body, "Start Date: 2024-01-01")
				assert.Contains(t, body, "Finish Date: 2024-01-25")
			},
		},
		{
			name: "payload with zero episodes",
			payload: domain.NotificationPayload{
				Event:           domain.NotificationEventSuccess,
				PlexEvent:       domain.PlexScrobbleEvent,
				EpisodesWatched: 0,
			},
			validate: func(t *testing.T, body string) {
				assert.NotContains(t, body, "Episodes Watched: 0")
				// Should still have some content (PlexEvent)
				assert.Contains(t, body, "New Plex Event:")
			},
		},
		{
			name: "payload with zero score",
			payload: domain.NotificationPayload{
				Event:     domain.NotificationEventSuccess,
				PlexEvent: domain.PlexScrobbleEvent,
				Score:     0,
			},
			validate: func(t *testing.T, body string) {
				assert.NotContains(t, body, "Score: 0")
				// Should still have some content (PlexEvent)
				assert.Contains(t, body, "New Plex Event:")
			},
		},
	}

	builder := &MessageBuilderPlainText{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builder.BuildBody(tt.payload)
			assert.NotEmpty(t, result)
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestMessageBuilderHTML_BuildBody(t *testing.T) {
	tests := []struct {
		name     string
		payload  domain.NotificationPayload
		validate func(*testing.T, string)
	}{
		{
			name:    "full payload with HTML escaping",
			payload: testdata.NewMockNotificationPayload(),
			validate: func(t *testing.T, body string) {
				assert.Contains(t, body, "<b>")
				assert.Contains(t, body, "Attack on Titan")
				assert.Contains(t, body, "Episodes Watched:")
				assert.Contains(t, body, "MAL Watch Status:")
			},
		},
		{
			name: "payload with HTML special characters",
			payload: domain.NotificationPayload{
				Subject:   "Test & Subject",
				Message:   "Message <script>alert('xss')</script>",
				MediaName: "Show & Title",
				Event:     domain.NotificationEventSuccess,
			},
			validate: func(t *testing.T, body string) {
				// Should escape HTML
				assert.Contains(t, body, "&amp;")
				assert.Contains(t, body, "&lt;")
				assert.Contains(t, body, "&gt;")
				assert.NotContains(t, body, "<script>")
			},
		},
		{
			name: "payload with humanized numbers",
			payload: domain.NotificationPayload{
				Event:           domain.NotificationEventSuccess,
				EpisodesWatched: 1234,
				Score:           5678,
				TimesRewatched:  999,
			},
			validate: func(t *testing.T, body string) {
				// Should use humanize.Comma for large numbers
				assert.Contains(t, body, "Episodes Watched:")
				assert.Contains(t, body, "Score:")
				assert.Contains(t, body, "Times Rewatched:")
			},
		},
		{
			name: "empty payload",
			payload: domain.NotificationPayload{
				Event:     domain.NotificationEventSuccess,
				PlexEvent: domain.PlexScrobbleEvent,
			},
			validate: func(t *testing.T, body string) {
				// Should still have some content
				assert.Contains(t, body, "New Plex Event:")
				assert.NotEmpty(t, body)
			},
		},
	}

	builder := &MessageBuilderHTML{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builder.BuildBody(tt.payload)
			assert.NotEmpty(t, result)
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestFormatMessageContent(t *testing.T) {
	tests := []struct {
		name         string
		messageParts []ConditionMessagePart
		expected     string
	}{
		{
			name: "all conditions true",
			messageParts: []ConditionMessagePart{
				{true, "First\n", []interface{}{}},
				{true, "Second\n", []interface{}{}},
				{true, "Third\n", []interface{}{}},
			},
			expected: "First\nSecond\nThird\n",
		},
		{
			name: "some conditions false",
			messageParts: []ConditionMessagePart{
				{true, "First\n", []interface{}{}},
				{false, "Second\n", []interface{}{}},
				{true, "Third\n", []interface{}{}},
			},
			expected: "First\nThird\n",
		},
		{
			name: "all conditions false",
			messageParts: []ConditionMessagePart{
				{false, "First\n", []interface{}{}},
				{false, "Second\n", []interface{}{}},
			},
			expected: "",
		},
		{
			name:         "empty parts",
			messageParts: []ConditionMessagePart{},
			expected:     "",
		},
		{
			name: "with format arguments",
			messageParts: []ConditionMessagePart{
				{true, "Value: %v\n", []interface{}{42}},
				{true, "Name: %s\n", []interface{}{"test"}},
			},
			expected: "Value: 42\nName: test\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatMessageContent(tt.messageParts)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildTitle(t *testing.T) {
	tests := []struct {
		name     string
		event    domain.NotificationEvent
		expected string
	}{
		{
			name:     "app update available",
			event:    domain.NotificationEventAppUpdateAvailable,
			expected: "shinkro Update Available",
		},
		{
			name:     "success",
			event:    domain.NotificationEventSuccess,
			expected: "MAL Update Successful",
		},
		{
			name:     "plex processing error",
			event:    domain.NotificationEventPlexProcessingError,
			expected: "Plex Processing Error",
		},
		{
			name:     "anime update error",
			event:    domain.NotificationEventAnimeUpdateError,
			expected: "Anime Update Error",
		},
		{
			name:     "test event",
			event:    domain.NotificationEventTest,
			expected: "TEST",
		},
		{
			name:     "unknown event",
			event:    domain.NotificationEvent("UNKNOWN"),
			expected: "NEW EVENT",
		},
		{
			name:     "empty event",
			event:    domain.NotificationEvent(""),
			expected: "NEW EVENT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildTitle(tt.event)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMessageBuilderPlainText_EdgeCases(t *testing.T) {
	builder := &MessageBuilderPlainText{}

	t.Run("empty sender", func(t *testing.T) {
		payload := testdata.NewMockNotificationPayload()
		payload.Sender = ""
		result := builder.BuildBody(payload)
		assert.NotContains(t, result, "Sender:")
	})

	t.Run("only subject no message", func(t *testing.T) {
		payload := domain.NotificationPayload{
			Subject: "Only Subject",
			Event:   domain.NotificationEventSuccess,
		}
		result := builder.BuildBody(payload)
		// Subject without message should not appear in the format
		assert.NotContains(t, result, "Only Subject")
	})

	t.Run("newlines in content", func(t *testing.T) {
		payload := domain.NotificationPayload{
			Subject: "Subject\nWith\nNewlines",
			Message: "Message\nWith\nNewlines",
			Event:   domain.NotificationEventSuccess,
		}
		result := builder.BuildBody(payload)
		// Should handle newlines properly
		assert.Contains(t, result, "Subject")
		assert.Contains(t, result, "Message")
	})
}

func TestMessageBuilderHTML_EdgeCases(t *testing.T) {
	builder := &MessageBuilderHTML{}

	t.Run("XSS prevention", func(t *testing.T) {
		payload := domain.NotificationPayload{
			Subject:   "<script>alert('xss')</script>",
			Message:   "<img src=x onerror=alert(1)>",
			MediaName: "Test & <b>Bold</b>",
			Event:     domain.NotificationEventSuccess,
		}
		result := builder.BuildBody(payload)
		// Should escape user-provided HTML - check that user HTML tags are escaped
		// Note: The builder uses <b> tags for its own formatting, so those are fine
		assert.NotContains(t, result, "<script>")
		assert.NotContains(t, result, "</script>")
		assert.NotContains(t, result, "<img")
		// User's <b>Bold</b> should be escaped, but builder's <b> tags are fine
		assert.Contains(t, result, "&lt;b&gt;Bold&lt;/b&gt;") // User's <b> is escaped
		// Check that escaped versions exist
		assert.Contains(t, result, "&lt;script&gt;")
		assert.Contains(t, result, "&lt;img")
		assert.Contains(t, result, "&amp;")
		// Builder's formatting tags should still be present
		assert.Contains(t, result, "<b>Show:</b>") // Builder's formatting is fine
	})

	t.Run("large numbers formatting", func(t *testing.T) {
		payload := domain.NotificationPayload{
			Event:           domain.NotificationEventSuccess,
			EpisodesWatched: 1234567,
			Score:           9876543,
			TimesRewatched:  12345,
		}
		result := builder.BuildBody(payload)
		// Should use humanize.Comma
		assert.Contains(t, result, "Episodes Watched:")
		// Check that numbers are formatted (humanize adds commas)
		lines := strings.Split(result, "\n")
		hasFormattedNumber := false
		for _, line := range lines {
			if strings.Contains(line, "Episodes Watched:") {
				// Should have comma in large number
				if strings.Contains(line, ",") {
					hasFormattedNumber = true
					break
				}
			}
		}
		assert.True(t, hasFormattedNumber, "should format large numbers with commas")
	})
}
