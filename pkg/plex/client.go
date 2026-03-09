package plex

import (
	"io"
	"log"
	"net/http"
	"time"

	"github.com/shinkro/shinkro/pkg/sharedhttp"
)

type Config struct {
	Url           string
	Token         string
	ClientID      string
	TLSSkipVerify bool
	Log           *log.Logger
}

type Client struct {
	config Config
	http   *http.Client

	Log *log.Logger
}

type tokenTransport struct {
	base     http.RoundTripper
	token    string
	clientID string
}

func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", sharedhttp.UserAgent)
	req.Header.Set("X-Plex-Token", t.token)
	req.Header.Set("X-Plex-Product", "shinkro")
	req.Header.Set("X-Plex-Client-Identifier", t.clientID)
	return t.base.RoundTrip(req)
}

func NewClient(config Config) *Client {
	httpClient := &http.Client{
		Timeout:   time.Second * 5,
		Transport: sharedhttp.Transport,
	}

	if config.TLSSkipVerify {
		httpClient.Transport = sharedhttp.TransportTLSInsecure
	}

	httpClient.Transport = &tokenTransport{
		base:     httpClient.Transport,
		token:    config.Token,
		clientID: config.ClientID,
	}

	c := &Client{
		config: config,
		http:   httpClient,
		Log:    log.New(io.Discard, "", log.LstdFlags),
	}

	if config.Log != nil {
		c.Log = config.Log
	}

	return c
}
