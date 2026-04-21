package render

import (
	"io"
	"strings"
	"sync"
	"time"

	"github.com/JacobJoergensen/preflight/internal/engine"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

const (
	spinnerFrameInterval = 80 * time.Millisecond
	progressSpinnerLabel = "installing…"
	ansiClearLine        = "\r\x1b[2K"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type TTYFixProgress struct {
	out          *terminal.OutputWriter
	nameWidth    int
	commandWidth int

	writeMu      sync.Mutex
	headerOnce   sync.Once
	currentStart time.Time
	currentName  string
	stopSignal   chan struct{}
	spinnerDone  chan struct{}
}

func NewTTYFixProgress(writer io.Writer) *TTYFixProgress {
	return &TTYFixProgress{out: terminal.NewOutputWriterFor(writer)}
}

func (p *TTYFixProgress) Plan(candidates []engine.FixCandidate) {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()

	for _, candidate := range candidates {
		if n := len(candidate.DisplayName); n > p.nameWidth {
			p.nameWidth = n
		}

		if n := len(progressCommandLabel(candidate)); n > p.commandWidth {
			p.commandWidth = n
		}
	}
}

func (p *TTYFixProgress) Start(candidate engine.FixCandidate) {
	p.headerOnce.Do(p.writeResultsHeader)

	p.writeMu.Lock()
	p.currentStart = time.Now()
	p.currentName = candidate.DisplayName
	p.stopSignal = make(chan struct{})
	p.spinnerDone = make(chan struct{})
	p.writeMu.Unlock()

	p.paintSpinnerLine(0)

	go p.tick()
}

func (p *TTYFixProgress) Finish(item result.FixItem) {
	p.writeMu.Lock()
	stop := p.stopSignal
	done := p.spinnerDone
	p.writeMu.Unlock()

	if stop != nil {
		close(stop)
		<-done
	}

	p.writeMu.Lock()
	defer p.writeMu.Unlock()

	p.writeFinalLineLocked(item)

	if !item.Success {
		p.writeFailureOutputLocked(item)
	}
}

func (p *TTYFixProgress) tick() {
	p.writeMu.Lock()
	stop := p.stopSignal
	done := p.spinnerDone
	p.writeMu.Unlock()

	defer close(done)

	ticker := time.NewTicker(spinnerFrameInterval)
	defer ticker.Stop()

	frame := 1

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			p.paintSpinnerLine(frame)
			frame = (frame + 1) % len(spinnerFrames)
		}
	}
}

func (p *TTYFixProgress) paintSpinnerLine(frame int) {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()

	elapsed := time.Since(p.currentStart)

	p.out.Printf("%s  %s%s%s  %s%s%s  %s%s%s  %s%s%s",
		ansiClearLine,
		terminal.Cyan, spinnerFrames[frame], terminal.Reset,
		terminal.Bold, padRight(p.currentName, p.nameWidth), terminal.Reset,
		terminal.Dim, padRight(progressSpinnerLabel, p.commandWidth), terminal.Reset,
		terminal.Dim, formatFixElapsed(elapsed), terminal.Reset,
	)
}

func (p *TTYFixProgress) writeFinalLineLocked(item result.FixItem) {
	icon, color := fixItemIcon(item)
	command := buildFullCommand(item.ManagerCommand, item.Args)

	if command == "" {
		command = fixItemNoCommandLabel
	}

	p.out.Printf("%s  %s%s%s  %s%s%s  %s%s%s  %s%s%s\n",
		ansiClearLine,
		color, icon, terminal.Reset,
		terminal.Bold, padRight(item.ManagerName, p.nameWidth), terminal.Reset,
		terminal.Dim, padRight(command, p.commandWidth), terminal.Reset,
		terminal.Dim, formatFixElapsed(item.EndedAt.Sub(item.StartedAt)), terminal.Reset,
	)
}

func (p *TTYFixProgress) writeFailureOutputLocked(item result.FixItem) {
	if strings.TrimSpace(item.Output) != "" {
		lines := capturedOutputLines(item.Output)

		p.out.PrintNewLines(1)

		indent := strings.Repeat(" ", fixFailureOutputIndent)

		for _, line := range lines {
			p.out.Printf("%s%s%s%s\n", terminal.Red+terminal.Dim, indent, line, terminal.Reset)
		}

		p.out.PrintNewLines(1)

		return
	}

	if item.Error != "" {
		p.out.Printf("%s%s%s%s\n",
			terminal.Red, strings.Repeat(" ", ttyProjectBodySpaces),
			item.Error, terminal.Reset,
		)
	}
}

func (p *TTYFixProgress) writeResultsHeader() {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()

	p.out.PrintNewLines(1)
	p.out.Println(terminal.Bold + "Results" + terminal.Reset)
	p.out.Println(terminal.Gray + strings.Repeat("─", fixResultsRuleWidth) + terminal.Reset)
}

func progressCommandLabel(candidate engine.FixCandidate) string {
	if candidate.Command != "" {
		return candidate.Command
	}

	return progressSpinnerLabel
}
