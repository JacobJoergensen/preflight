package release

import (
	"runtime/debug"
	"strings"
)

var (
	Version string
	Commit  string
	Date    string
)

func BuildInfo() (version, commit, date string, dirty bool) {
	version, commit, date = Version, Commit, Date

	if version != "" && commit != "" && date != "" {
		return version, commit, date, false
	}

	info, ok := debug.ReadBuildInfo()

	if !ok {
		return version, commit, date, false
	}

	if version == "" {
		version = releaseVersion(info.Main.Version)
	}

	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			if commit == "" {
				commit = shortCommit(setting.Value)
			}
		case "vcs.time":
			if date == "" {
				date = setting.Value
			}
		case "vcs.modified":
			dirty = setting.Value == "true"
		}
	}

	return version, commit, date, dirty
}

func releaseVersion(mainVersion string) string {
	version := strings.TrimPrefix(mainVersion, "v")

	if version == "" || version == "(devel)" || strings.HasPrefix(version, "0.0.0-") {
		return ""
	}

	return version
}

func shortCommit(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}

	return commit
}
