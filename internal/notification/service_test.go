package notification

import (
	"testing"

	"github.com/shinkro/shinkro/internal/domain"

	"github.com/stretchr/testify/assert"
)

func TestEnabledEvent(t *testing.T) {
	tests := []struct {
		name     string
		events   []string
		event    domain.NotificationEvent
		expected bool
	}{
		{
			name:     "exact match",
			events:   []string{"SUCCESS", "TEST"},
			event:    domain.NotificationEventSuccess,
			expected: true,
		},
		{
			name:     "no match",
			events:   []string{"SUCCESS", "TEST"},
			event:    domain.NotificationEventAppUpdateAvailable,
			expected: false,
		},
		{
			name:     "ERROR matches PlexProcessingError",
			events:   []string{"ERROR"},
			event:    domain.NotificationEventPlexProcessingError,
			expected: true,
		},
		{
			name:     "ERROR matches AnimeUpdateError",
			events:   []string{"ERROR"},
			event:    domain.NotificationEventAnimeUpdateError,
			expected: true,
		},
		{
			name:     "ERROR does not match other events",
			events:   []string{"ERROR"},
			event:    domain.NotificationEventSuccess,
			expected: false,
		},
		{
			name:     "empty events list",
			events:   []string{},
			event:    domain.NotificationEventSuccess,
			expected: false,
		},
		{
			name:     "multiple events including match",
			events:   []string{"SUCCESS", "ERROR", "TEST"},
			event:    domain.NotificationEventPlexProcessingError,
			expected: true,
		},
		{
			name:     "case sensitive match",
			events:   []string{"success"}, // lowercase
			event:    domain.NotificationEventSuccess,
			expected: false, // Should be case sensitive
		},
		{
			name:     "exact error type match takes precedence",
			events:   []string{"PLEX_PROCESSING_ERROR", "ERROR"},
			event:    domain.NotificationEventPlexProcessingError,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := enabledEvent(tt.events, tt.event)
			assert.Equal(t, tt.expected, result)
		})
	}
}

