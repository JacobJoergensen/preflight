package cmd

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/release"
	"github.com/JacobJoergensen/preflight/internal/render"
)

func platform() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

var Version = "1.0.0"

type versionOptions struct {
	json bool
}

var versionOpts versionOptions

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Shows PreFlight version information",
	Long:  `Shows detailed information about the PreFlight version including version number and build date.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		startedAt := time.Now()
		versionData, done := release.GetVersionInfo(Version, platform())

		<-done

		report := result.VersionReport{
			StartedAt:     startedAt,
			EndedAt:       time.Now(),
			Version:       versionData.Version,
			Platform:      versionData.Platform,
			LatestVersion: versionData.LatestVersion,
			HasUpdate:     versionData.HasUpdate,
			CheckFailed:   versionData.Error != nil,
		}

		if versionData.Error != nil {
			report.CheckErrorText = fmt.Sprintf("%v", versionData.Error)
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
