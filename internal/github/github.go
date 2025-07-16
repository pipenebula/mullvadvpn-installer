package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"
)

const apiURL = "https://api.github.com/repos/mullvad/mullvadvpn-app/releases?per_page=50"

var (
	androidTagRE = regexp.MustCompile(`(?i)^android/`)
	betaTagRE    = regexp.MustCompile(`-beta\d*$`)
	anyBetaRE    = regexp.MustCompile(`-beta`)
)

type Release struct {
	Tag    string
	Assets []Asset
}

type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type rawRelease struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

func GetLatestRelease(channel string) (*Release, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d from GitHub API", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var raws []rawRelease
	if err := json.Unmarshal(data, &raws); err != nil {
		return nil, fmt.Errorf("unmarshal JSON: %w", err)
	}

	for _, r := range raws {
		if filterChannel(r.TagName, channel) {
			return &Release{Tag: r.TagName, Assets: r.Assets}, nil
		}
	}
	return nil, fmt.Errorf("no %q release found", channel)
}

func filterChannel(tag, channel string) bool {
	if androidTagRE.MatchString(tag) {
		return false
	}
	switch channel {
	case "stable":
		return !anyBetaRE.MatchString(tag)
	case "beta":
		return betaTagRE.MatchString(tag)
	default:
		return false
	}
}
