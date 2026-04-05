package result

import "time"

type VersionReport struct {
	StartedAt      time.Time
	EndedAt        time.Time
	Version        string
	Platform       string
	LatestVersion  string
	HasUpdate      bool
	CheckFailed    bool
	CheckErrorText string
}
