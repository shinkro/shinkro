package update

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/shinkro/shinkro/pkg/sharedhttp"
)

// LatestTag returns the latest GitHub release tag for shinkro
func LatestTag(ctx context.Context) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second, Transport: sharedhttp.Transport}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/shinkro/shinkro/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", sharedhttp.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	return body.TagName, nil
}
