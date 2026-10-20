package plex

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
)

func (c *Client) GetShowID(ctx context.Context, key string) (*GUID, error) {
	baseUrl, err := url.Parse(c.config.Url)
	if err != nil {
		c.Log.Print("method: getShowID, error: ", err)
		return nil, errors.Wrap(err, "plex url invalid")
	}

	baseUrl = baseUrl.JoinPath(key)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseUrl.String(), nil)
	if err != nil {
		c.Log.Print("method: getShowID, error: ", err)
		return nil, errors.Wrap(err, "plex request invalid")
	}

	var plexResp PlexResponse
	req.Header.Add("ContainerStart", "X-Plex-Container-Start=0")
	req.Header.Add("ContainerSize", "Plex-Container-Size=100")

	resp, err := c.http.Do(req)
	if err != nil {
		c.Log.Print("method: getShowID, error: ", err)
		return nil, errors.Wrap(err, "network error")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		err = errors.Errorf("%v, response status: %v, response body: %v", err, resp.StatusCode, string(body))
		c.Log.Print("method: getShowID, error: ", err)
		return nil, err
	}

	defer resp.Body.Close()
	err = json.Unmarshal(body, &plexResp)
	if err != nil {
		err = errors.Errorf("%v, response status: %v, response body: %v", err, resp.StatusCode, string(body))
		c.Log.Print("method: getShowID, error: ", err)
		return nil, err
	}

	if len(plexResp.MediaContainer.Metadata) != 1 {
		err = errors.Errorf("something went wrong in getting guid from plex:%v, response status: %v, response body: %v", err, resp.StatusCode, string(body))
		c.Log.Print("method: getShowID, error: ", err)
		return nil, err
	}

	return &plexResp.MediaContainer.Metadata[0].GUID, nil
}

func (c *Client) TestToken(ctx context.Context) error {
	checkUrl := "https://plex.tv/api/v2/user"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkUrl, nil)
	if err != nil {
		c.Log.Print("method: test, error: ", err)
		return errors.Wrap(err, "plex request invalid")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		c.Log.Print("method: test, error: ", err)
		return errors.Wrap(err, "network error")
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = errors.New("plex access token invalid")
		c.Log.Print("method: test,  error: ", err)
		return err
	}

	return nil
}

func (c *Client) TestConnection(ctx context.Context) error {
	err := c.TestToken(ctx)
	if err != nil {
		c.Log.Print("method: testConnection, error: ", err)
		return errors.Wrap(err, "plex token invalid")
	}

	baseUrl, err := url.Parse(c.config.Url)
	if err != nil {
		c.Log.Print("method: testConnection, error: ", err)
		return errors.Wrap(err, "plex url invalid")
	}

	baseUrl = baseUrl.JoinPath("/myplex/account")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseUrl.String(), nil)
	if err != nil {
		c.Log.Print("method: testConnection, error: ", err)
		return errors.Wrap(err, "plex request invalid")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		c.Log.Print("method: testConnection, error: ", err)
		return errors.Wrap(err, "network error")
	}

	if resp.StatusCode == http.StatusUnauthorized {
		err = errors.New("unauthorized: check plex token")
		c.Log.Print("method: testConnection, error: ", err)
		return err
	}

	if resp.StatusCode == http.StatusBadRequest {
		err = errors.New("bad request: check plex client identifier")
		c.Log.Print("method: testConnection, error: ", err)
		return err
	}

	if resp.StatusCode != http.StatusOK {
		err = errors.New("woops plex won't let us connect")
		c.Log.Print("method: testConnection, error: ", err)
		return err
	}

	return nil
}

func (c *Client) GetServerList(ctx context.Context) (*ServerResponse, error) {
	plexUrl := "https://plex.tv/api/v2/resources"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, plexUrl, nil)
	if err != nil {
		c.Log.Print("method: getServerList, error: ", err)
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		c.Log.Print("method: getServerList, error: ", err)
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.Log.Print("method: getServerList, error: ", err)
		return nil, errors.Wrap(err, "error reading response body")
	}

	if resp.StatusCode == http.StatusUnauthorized {
		err = errors.New("unauthorized check plex token")
		c.Log.Printf("method: getServerList, error: %v, response: %v", err, string(body))
		return nil, err
	}

	if resp.StatusCode == http.StatusBadRequest {
		err = errors.New("bad request check plex client identifier")
		c.Log.Printf("method: getServerList, error: %v, response: %v", err, string(body))
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		err = errors.New("unknown or invalid Plex response")
		c.Log.Printf("method: getServerList, error: %v, response: %v", err, string(body))
		return nil, err
	}

	var servers []Server
	if err := json.Unmarshal(body, &servers); err != nil {
		c.Log.Print("method: getServerList, error: ", err)
		return nil, errors.Wrap(err, "error decoding server list response body")
	}

	if len(servers) <= 0 {
		err = errors.New("no servers found")
		c.Log.Print("method: getServerList, error: ", err)
		return nil, err
	}

	return &ServerResponse{Servers: servers}, nil
}

func (c *Client) GetLibraries(ctx context.Context) (*LibraryResponse, error) {
	libUrl, err := url.Parse(c.config.Url)
	if err != nil {
		c.Log.Print("method: getLibraries, error: ", err)
		return nil, errors.Wrap(err, "plex url invalid")
	}

	libUrl = libUrl.JoinPath("/library/sections")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, libUrl.String(), nil)
	if err != nil {
		c.Log.Print("method: getLibraries, error: ", err)
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		c.Log.Print("method: getLibraries, error: ", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.Log.Print("method: getLibraries, error: ", err)
		return nil, errors.Wrap(err, "error reading response body")
	}

	if resp.StatusCode == http.StatusUnauthorized {
		err = errors.New("unauthorized: check plex token")
		c.Log.Printf("method: getLibraries, error: %v, response: %v", err, string(body))
		return nil, err
	}

	if resp.StatusCode == http.StatusBadRequest {
		err = errors.New("bad request: check plex client identifier")
		c.Log.Printf("method: getLibraries, error: %v, response: %v", err, string(body))
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		err = errors.New("unknown or invalid Plex response")
		c.Log.Printf("method: getLibraries, error: %v, response: %v", err, string(body))
		return nil, err
	}

	var libResp LibraryResponse
	if err := json.Unmarshal(body, &libResp); err != nil {
		c.Log.Print("method: getLibraries, error: ", err)
		return nil, errors.Wrap(err, "error decoding library response body")
	}

	if len(libResp.MediaContainer.Directory) <= 0 {
		err = errors.New("no libraries found")
		c.Log.Print("method: getLibraries, error: ", err)
		return nil, err
	}

	return &libResp, nil
}

// Identity returns the machine identity of the configured server without
// exposing its address, credentials, or a complete provider response in errors.
func (c *Client) Identity(ctx context.Context) (string, error) {
	base, err := url.Parse(c.config.Url)
	if err != nil {
		return "", errors.New("configured Plex server URL is invalid")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.JoinPath("/identity").String(), nil)
	if err != nil {
		return "", errors.New("Plex identity request could not be created")
	}
	response, err := c.http.Do(req)
	if err != nil {
		return "", errors.New("Plex identity request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", errors.New("Plex identity request was rejected")
	}
	var result struct {
		MediaContainer struct {
			MachineIdentifier string `json:"machineIdentifier"`
		} `json:"MediaContainer"`
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil || result.MediaContainer.MachineIdentifier == "" {
		return "", errors.New("Plex identity response was invalid")
	}
	return result.MediaContainer.MachineIdentifier, nil
}
