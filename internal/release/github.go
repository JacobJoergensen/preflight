package release

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type VersionData struct {
	Version       string
	LatestVersion string
	Platform      string
	ReleaseURL    string
	HasUpdate     bool
}

type githubRelease struct {
	TagName string `json:"tag_name"`
}

func FetchLatestRelease(ctx context.Context, owner, repo string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	client := &http.Client{Timeout: 5 * time.Second}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	response, err := client.Do(request)

	if err != nil {
		return "", fmt.Errorf("fetching release: %w", err)
	}

	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status: %s", response.Status)
	}

	body, err := io.ReadAll(response.Body)

	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	var release githubRelease

	if err := json.Unmarshal(body, &release); err != nil {
		return "", fmt.Errorf("decoding JSON: %w", err)
	}

	if release.TagName == "" {
		return "", errors.New("no release found in GitHub repo")
	}

	return release.TagName, nil
}

func FetchVersionInfo(currentVersion, platform string) (VersionData, error) {
	latest, err := FetchLatestRelease(context.Background(), "JacobJoergensen", "preflight")

	if err != nil {
		return VersionData{
			Version:  currentVersion,
			Platform: platform,
		}, err
	}

	hasUpdate := currentVersion != latest
	releaseURL := ""

	if hasUpdate {
		releaseURL = "https://github.com/JacobJoergensen/preflight/releases/tag/" + latest
	}

	return VersionData{
		Version:       currentVersion,
		LatestVersion: latest,
		Platform:      platform,
		ReleaseURL:    releaseURL,
		HasUpdate:     hasUpdate,
	}, nil
}
