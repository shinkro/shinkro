package syncinspector

import (
	"context"
	"github.com/shinkro/shinkro/internal/domain"
)

// Eligibility is read again before preview, queueing and execution. Persisted
// input is evidence of receipt, never a permanent grant to update an account.
type Eligibility func(context.Context, Input) ([]string, error)

func (s *Service) SetEligibility(check Eligibility) { s.eligibility = check }
func (s *Service) currentEligibility(ctx context.Context, in Input) ([]string, error) {
	if s.eligibility == nil {
		return nil, fail("ELIGIBILITY_UNAVAILABLE", "Current Plex user and library eligibility could not be verified. Check Plex settings before retrying.")
	}
	warnings, err := s.eligibility(ctx, in)
	if err != nil {
		if _, ok := err.(*Error); !ok {
			return warnings, fail("ELIGIBILITY_UNAVAILABLE", "Current Plex eligibility could not be verified. Check Plex settings before retrying.")
		}
	}
	return warnings, err
}

type PlexEligibility struct {
	Settings func(context.Context) (*domain.PlexSettings, error)
	Identity func(context.Context, *domain.PlexSettings) (string, error)
}

func (c PlexEligibility) Check(ctx context.Context, in Input) ([]string, error) {
	warnings := []string{}
	if c.Settings == nil {
		return warnings, fail("ELIGIBILITY_UNAVAILABLE", "Current Plex settings could not be read.")
	}
	settings, err := c.Settings(ctx)
	if err != nil || settings == nil {
		return warnings, fail("ELIGIBILITY_UNAVAILABLE", "Current Plex settings could not be read. Configure the Plex user and anime libraries before retrying.")
	}
	if settings.PlexUser == "" || settings.PlexUser != in.Account {
		return warnings, fail("EVENT_NOT_ELIGIBLE", "This event's Plex user is no longer enabled in the current settings. It will not update MAL.")
	}
	allowed := false
	for _, library := range settings.AnimeLibraries {
		if library == in.Library {
			allowed = true
			break
		}
	}
	if !allowed {
		return warnings, fail("EVENT_NOT_ELIGIBLE", "This event's library is no longer enabled as an anime library. It will not update MAL.")
	}
	if in.ServerID == "" {
		warnings = append(warnings, "This event has no recorded Plex server identity. Current user and library eligibility were verified; server identity could not be compared.")
		return warnings, nil
	}
	if !settings.PlexClientEnabled || settings.Host == "" {
		warnings = append(warnings, "The Plex API client is disabled or unconfigured. Current user and library eligibility were verified; the recorded server identity could not be compared.")
		return warnings, nil
	}
	if c.Identity == nil {
		return warnings, fail("ELIGIBILITY_UNAVAILABLE", "The configured Plex server identity could not be verified. Check the Plex connection before retrying.")
	}
	current, err := c.Identity(ctx, settings)
	if err != nil || current == "" {
		return warnings, fail("ELIGIBILITY_UNAVAILABLE", "The configured Plex server identity could not be verified. Check the Plex connection before retrying.")
	}
	if current != in.ServerID {
		return warnings, fail("EVENT_NOT_ELIGIBLE", "This event belongs to a different Plex server than the one currently configured. It will not update MAL.")
	}
	return warnings, nil
}
