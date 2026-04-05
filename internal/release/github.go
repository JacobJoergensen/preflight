package release

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type VersionData struct {
	Version       string
	LatestVersion string
	Platform      string
	HasUpdate     bool
	Error         error
}

type GitHubRelease struct {
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

	defer func() {
		if err := response.Body.Close(); err != nil {
			slog.Error("error closing response body", "error", err)
		}
	}()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status: %s", response.Status)
	}

	body, err := io.ReadAll(response.Body)

	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	var release GitHubRelease

	if err := json.Unmarshal(body, &release); err != nil {
		return "", fmt.Errorf("decoding JSON: %w", err)
	}

	if release.TagName == "" {
		return "", errors.New("no release found in GitHub repo")
	}

	return release.TagName, nil
}

func GetVersionInfo(currentVersion, platform string) (*VersionData, chan bool) {
	info := &VersionData{
		Version:  currentVersion,
		Platform: platform,
	}

	done := make(chan bool)

	go func() {
		defer close(done)

		latest, err := FetchLatestRelease(context.Background(), "JacobJoergensen", "preflight")

		if err != nil {
			info.Error = err
			info.LatestVersion = "Unable to check"
			return
		}

		info.LatestVersion = latest
		info.HasUpdate = currentVersion != latest
	}()

	return info, done
}
