package result

import "time"

type VersionReport struct {
	StartedAt      time.Time
	EndedAt        time.Time
	Version        string
	Commit         string
	BuildDate      string
	Platform       string
	LatestVersion  string
	ReleaseURL     string
	HasUpdate      bool
	CheckErrorText string
}
