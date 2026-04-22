package release

import (
	"runtime/debug"
	"strings"
)

var (
	Version = ""
	Commit  = ""
	Date    = ""
)

func BuildInfo() (version, commit, date string) {
	version, commit, date = Version, Commit, Date

	if version != "" && commit != "" && date != "" {
		return version, commit, date
	}

	info, ok := debug.ReadBuildInfo()

	if !ok {
		return version, commit, date
	}

	if version == "" && info.Main.Version != "" {
		version = strings.TrimPrefix(info.Main.Version, "v")
	}

	for _, setting := range info.Settings {
		if commit == "" && setting.Key == "vcs.revision" {
			commit = setting.Value

			if len(commit) > 7 {
				commit = commit[:7]
			}
		}

		if date == "" && setting.Key == "vcs.time" {
			date = setting.Value
		}
	}

	return version, commit, date
}
