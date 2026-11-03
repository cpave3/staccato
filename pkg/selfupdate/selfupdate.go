// Package selfupdate finds a program's latest stable release on GitHub and
// puts its binary in place of the running one. It knows nothing of st beyond
// the names of the release's files.
//
// It is a copy of Legato's internal/engine/selfupdate, kept in the same shape
// so that a fix in one is easy to carry to the other. The differences: the
// asset prefix is a field of Client, and an unwritable directory is named.
package selfupdate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// ErrNoRelease is a repository with no stable release.
var ErrNoRelease = errors.New("no release has been published")

// Client reads releases of one GitHub repository.
type Client struct {
	// API is GitHub's API address; "" for https://api.github.com.
	API string
	// Repo is owner/name.
	Repo string
	// Asset is the prefix of the release's binaries, named
	// <Asset>-<os>-<arch>.
	Asset string
	// HTTP is the client to use; nil for http.DefaultClient.
	HTTP *http.Client
}

// Release is a published release.
type Release struct {
	// Version is the release's tag, such as v1.2.3.
	Version string
	// URL is the release's page.
	URL string
	// Assets are the release's files: name to download address.
	Assets map[string]string
}

// Latest is the repository's newest stable release: GitHub's "latest", which
// is never a draft or a prerelease.
func (c Client) Latest(ctx context.Context) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.api()+"/repos/"+c.Repo+"/releases/latest", nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := c.http().Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Release{}, ErrNoRelease
	}
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("GitHub answered %s for the latest release of %s", resp.Status, c.Repo)
	}
	var body struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Release{}, fmt.Errorf("reading the latest release of %s: %w", c.Repo, err)
	}
	rel := Release{Version: body.TagName, URL: body.HTMLURL, Assets: map[string]string{}}
	for _, a := range body.Assets {
		rel.Assets[a.Name] = a.URL
	}
	return rel, nil
}

func (c Client) api() string {
	if c.API == "" {
		return "https://api.github.com"
	}
	return c.API
}

func (c Client) http() *http.Client {
	if c.HTTP == nil {
		return http.DefaultClient
	}
	return c.HTTP
}
