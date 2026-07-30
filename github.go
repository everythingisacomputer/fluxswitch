package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

const releasesURL = "https://api.github.com/repos/fluxcd/flux2/releases?per_page=%d"

// apiClient is for small API requests where a total deadline is appropriate.
// Release tarballs are fetched with downloadClient instead, which must not
// cap total transfer time.
var apiClient = &http.Client{Timeout: 30 * time.Second}

// downloadClient bounds connection setup and server responsiveness but not
// the overall transfer, so large downloads survive slow links.
var downloadClient = &http.Client{
	Transport: &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		Proxy:                 http.ProxyFromEnvironment,
	},
}

// versionRE matches release versions like "2.9.3" or "2.0.0-rc.1". Version
// strings are used to build URLs and filesystem paths, so anything else
// (path separators, "..", tag names we don't expect) is rejected outright.
var versionRE = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z][0-9A-Za-z.]*)?$`)

func isValidVersion(v string) bool {
	return versionRE.MatchString(v)
}

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
	// An optional token raises GitHub's rate limit from 60 to 5000 req/hour.
	if token := githubToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := apiClient.Do(req)
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
		v := strings.TrimPrefix(r.TagName, "v")
		if isValidVersion(v) {
			versions = append(versions, v)
		}
	}
	return versions, nil
}

func githubToken() string {
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	return os.Getenv("GH_TOKEN")
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
