package render

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/JacobJoergensen/preflight/internal/terminal"
)

const (
	spinnerFrameInterval = 80 * time.Millisecond
	ansiClearLine        = "\r\x1b[2K"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type Spinner struct {
	stop chan struct{}
	done chan struct{}
}

type TTYProgress struct {
	out     *terminal.OutputWriter
	label   string
	mu      sync.Mutex
	spinner *Spinner
	running map[string]string
	done    int
	total   int
	started bool
}

func NewSpinner() *Spinner {
	return &Spinner{}
}

func (s *Spinner) Frame(i int) string {
	return spinnerFrames[i%len(spinnerFrames)]
}

func (s *Spinner) Start(paint func(frame string)) {
	s.stop = make(chan struct{})
	s.done = make(chan struct{})

	go s.tick(paint)
}

func (s *Spinner) Stop() {
	if s.stop == nil {
		return
	}

	close(s.stop)
	<-s.done

	s.stop = nil
	s.done = nil
}

func (s *Spinner) tick(paint func(frame string)) {
	defer close(s.done)

	ticker := time.NewTicker(spinnerFrameInterval)
	defer ticker.Stop()

	frame := 1

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			paint(spinnerFrames[frame%len(spinnerFrames)])
			frame++
		}
	}
}

func NewTTYProgress(writer io.Writer, label string) *TTYProgress {
	return &TTYProgress{
		out:     terminal.NewOutputWriterFor(writer),
		label:   label,
		spinner: NewSpinner(),
		running: make(map[string]string),
	}
}

func (p *TTYProgress) Plan(total int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.total += total
}

func (p *TTYProgress) Start(scopeID, displayName string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.running[scopeID] = displayName

	if p.started {
		return
	}

	p.started = true
	p.paintLocked(p.spinner.Frame(0))

	p.spinner.Start(func(frame string) {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.paintLocked(frame)
	})
}

func (p *TTYProgress) Finish(scopeID string, included bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.running[scopeID]; !ok {
		return
	}

	delete(p.running, scopeID)

	if included {
		p.done++
		return
	}

	p.total--
}

func (p *TTYProgress) Close() {
	p.spinner.Stop()

	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return
	}

	p.out.Printf("%s", ansiClearLine)
	p.started = false
}

func (p *TTYProgress) paintLocked(frame string) {
	names := slices.Sorted(maps.Values(p.running))

	line := fmt.Sprintf("%s  %s%s%s  %s%s%s  %s%d of %d%s",
		ansiClearLine,
		terminal.Cyan, frame, terminal.Reset,
		terminal.Bold, p.label, terminal.Reset,
		terminal.Dim, p.done, p.total, terminal.Reset,
	)

	if len(names) > 0 {
		line += fmt.Sprintf("  %s· %s%s", terminal.Dim, strings.Join(names, ", "), terminal.Reset)
	}

	p.out.Printf("%s", line)
}
