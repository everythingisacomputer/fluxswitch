package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const releasesURL = "https://api.github.com/repos/fluxcd/flux2/releases?per_page=%d"

var httpClient = &http.Client{Timeout: 30 * time.Second}

type release struct {
	TagName    string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

// AvailableVersions fetches recent stable Flux release versions from GitHub,
// newest first, without the leading "v".
func AvailableVersions(limit int) ([]string, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf(releasesURL, limit), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching releases: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching releases: GitHub API returned %s", resp.Status)
	}

	var releases []release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decoding releases: %w", err)
	}

	var versions []string
	for _, r := range releases {
		if r.Draft || r.Prerelease {
			continue
		}
		versions = append(versions, strings.TrimPrefix(r.TagName, "v"))
	}
	return versions, nil
}

func LatestVersion() (string, error) {
	versions, err := AvailableVersions(5)
	if err != nil {
		return "", err
	}
	if len(versions) == 0 {
		return "", fmt.Errorf("no stable releases found")
	}
	return versions[0], nil
}
