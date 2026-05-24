package ecosystem

import (
	"context"
	"errors"
	"io"
	"slices"
	"testing"

	"github.com/JacobJoergensen/preflight/internal/exec"
	"github.com/JacobJoergensen/preflight/internal/fs/memfs"
)

func TestResolveGlobMarker(t *testing.T) {
	spec := &Spec{
		Name:     "dotnet",
		Managers: []Manager{{Command: "dotnet"}},
		Detect:   []Marker{{Glob: "*.csproj", Manager: "dotnet"}},
	}

	t.Run("detects when a file matches the glob", func(t *testing.T) {
		rc := RunContext{FS: memfs.New(map[string][]byte{"App.csproj": nil})}

		detection, ok := spec.Resolve(rc)
		if !ok {
			t.Fatal("expected the project to be detected")
		}

		if detection.Active.Command != "dotnet" {
			t.Errorf("active manager = %q, want dotnet", detection.Active.Command)
		}
	})

	t.Run("no detection without a matching file", func(t *testing.T) {
		rc := RunContext{FS: memfs.New(map[string][]byte{"README.md": nil})}

		if _, ok := spec.Resolve(rc); ok {
			t.Error("expected no detection without a .csproj")
		}
	})
}

var errFake = errors.New("fake failure")

type fakeRunner struct {
	result exec.Result
	err    error
	calls  [][]string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (exec.Result, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	return f.result, f.err
}

type fakeStream struct {
	output string
	err    error
	calls  [][]string
}

func (f *fakeStream) RunStreaming(_ context.Context, name string, args []string, stdout, _ io.Writer) (exec.Result, error) {
	f.calls = append(f.calls, append([]string{name}, args...))

	if f.output != "" {
		_, _ = stdout.Write([]byte(f.output))
	}

	return exec.Result{}, f.err
}

func TestRunFix(t *testing.T) {
	manager := Manager{
		Command:     "npm",
		DisplayName: "NPM",
		VersionArgs: []string{"--version"},
		InstallArgs: []string{"install"},
		ForceArgs:   []string{"--force"},
	}

	tests := []struct {
		name             string
		options          FixOptions
		versionErr       bool
		streamErr        bool
		streamOutput     string
		wantSuccess      bool
		wantErr          bool
		wantWouldRun     string
		wantOutput       string
		wantArgs         []string
		wantStreamCalled bool
	}{
		{
			name:         "dry run reports the command without executing",
			options:      FixOptions{DryRun: true},
			wantSuccess:  true,
			wantWouldRun: "npm install",
			wantArgs:     []string{"install"},
		},
		{
			name:         "force appends the force args",
			options:      FixOptions{DryRun: true, Force: true},
			wantSuccess:  true,
			wantWouldRun: "npm install --force",
			wantArgs:     []string{"install", "--force"},
		},
		{
			name:       "version probe failure aborts before install",
			versionErr: true,
			wantErr:    true,
		},
		{
			name:             "successful install captures output",
			streamOutput:     "added 1 package",
			wantSuccess:      true,
			wantOutput:       "added 1 package",
			wantArgs:         []string{"install"},
			wantStreamCalled: true,
		},
		{
			name:             "install failure records error and output",
			streamErr:        true,
			streamOutput:     "boom",
			wantErr:          true,
			wantOutput:       "boom",
			wantStreamCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{result: exec.Result{Stdout: "10.0.0"}}
			stream := &fakeStream{output: tt.streamOutput}

			if tt.versionErr {
				runner.err = errFake
			}

			if tt.streamErr {
				stream.err = errFake
			}

			rc := RunContext{Runner: runner, Stream: stream}
			spec := &Spec{Name: "js"}

			item, err := spec.RunFix(context.Background(), rc, Detection{Active: manager}, tt.options)
			if err != nil {
				t.Fatalf("RunFix returned a non-nil error: %v", err)
			}

			if item.ScopeID != "js" || item.ManagerCommand != "npm" || item.ManagerName != "NPM" {
				t.Errorf("item metadata = {%q %q %q}, want {js npm NPM}", item.ScopeID, item.ManagerCommand, item.ManagerName)
			}

			if item.Success != tt.wantSuccess {
				t.Errorf("success = %v, want %v", item.Success, tt.wantSuccess)
			}

			if (item.Error != "") != tt.wantErr {
				t.Errorf("error = %q, want set=%v", item.Error, tt.wantErr)
			}

			if item.WouldRun != tt.wantWouldRun {
				t.Errorf("wouldRun = %q, want %q", item.WouldRun, tt.wantWouldRun)
			}

			if item.Output != tt.wantOutput {
				t.Errorf("output = %q, want %q", item.Output, tt.wantOutput)
			}

			if tt.wantArgs != nil && !slices.Equal(item.Args, tt.wantArgs) {
				t.Errorf("args = %v, want %v", item.Args, tt.wantArgs)
			}

			if streamCalled := len(stream.calls) > 0; streamCalled != tt.wantStreamCalled {
				t.Errorf("stream called = %v, want %v", streamCalled, tt.wantStreamCalled)
			}
		})
	}
}

func TestRunOutdated(t *testing.T) {
	parse := func(_ RunContext, _ string) ([]OutdatedPackage, error) {
		return []OutdatedPackage{{Name: "left-pad", Current: "1.0.0", Latest: "1.1.0"}}, nil
	}

	tests := []struct {
		name         string
		probe        *OutdatedProbe
		runnerStdout string
		runnerErr    bool
		wantCommand  string
		wantErr      bool
		wantCount    int
	}{
		{
			name:        "no probe returns nothing without running",
			probe:       nil,
			wantCommand: "",
			wantCount:   0,
		},
		{
			name:         "empty tool defaults to the manager command",
			probe:        &OutdatedProbe{Args: []string{"outdated", "--json"}, Parse: parse},
			runnerStdout: "{}",
			wantCommand:  "npm",
			wantCount:    1,
		},
		{
			name:         "explicit tool overrides the manager command",
			probe:        &OutdatedProbe{Tool: "custom-tool", Args: []string{"list"}, Parse: parse},
			runnerStdout: "{}",
			wantCommand:  "custom-tool",
			wantCount:    1,
		},
		{
			name:      "error with no output is returned",
			probe:     &OutdatedProbe{Args: []string{"outdated"}, Parse: parse},
			runnerErr: true,
			wantErr:   true,
		},
		{
			name:         "error with output still parses",
			probe:        &OutdatedProbe{Args: []string{"outdated"}, Parse: parse},
			runnerStdout: "{}",
			runnerErr:    true,
			wantCommand:  "npm",
			wantCount:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{result: exec.Result{Stdout: tt.runnerStdout}}

			if tt.runnerErr {
				runner.err = errFake
			}

			manager := Manager{Command: "npm", Outdated: tt.probe}
			spec := &Spec{Name: "js"}

			packages, err := spec.RunOutdated(context.Background(), RunContext{Runner: runner}, Detection{Active: manager})

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(packages) != tt.wantCount {
				t.Errorf("packages = %d, want %d", len(packages), tt.wantCount)
			}

			gotCommand := ""

			if len(runner.calls) > 0 {
				gotCommand = runner.calls[0][0]
			}

			if gotCommand != tt.wantCommand {
				t.Errorf("ran command %q, want %q", gotCommand, tt.wantCommand)
			}
		})
	}
}
