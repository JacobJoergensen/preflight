package cmd

import (
	"os"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/release"
	"github.com/JacobJoergensen/preflight/internal/render"
)

type versionOptions struct {
	json bool
}

var versionOpts versionOptions

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Shows PreFlight version information",
	Long:  `Shows the installed version, build metadata (commit, build date), platform, and checks GitHub for available updates.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		startedAt := time.Now()
		version, commit, buildDate := release.BuildInfo()
		versionData, err := release.FetchVersionInfo(version, runtime.GOOS+"/"+runtime.GOARCH)

		report := result.VersionReport{
			StartedAt:     startedAt,
			EndedAt:       time.Now(),
			Version:       versionData.Version,
			Commit:        commit,
			BuildDate:     buildDate,
			Platform:      versionData.Platform,
			LatestVersion: versionData.LatestVersion,
			ReleaseURL:    versionData.ReleaseURL,
			HasUpdate:     versionData.HasUpdate,
		}

		if err != nil {
			report.CheckErrorText = err.Error()
		}

		if versionOpts.json {
			return render.JSONVersionRenderer{Out: os.Stdout}.Render(report)
		}

		return render.TTYVersionRenderer{}.Render(report)
	},
}

func init() {
	versionCmd.Flags().BoolVar(&versionOpts.json, "json", false, "Output results as JSON")
	rootCmd.AddCommand(versionCmd)
}
