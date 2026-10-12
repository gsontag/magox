package report

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/pterm/pterm"
)

// screen is the area of the terminal that the steps are drawn in.
type screen interface {
	Update(text ...any)
	Stop() error
}

// newScreen opens an area; the tests replace it, pterm needing a terminal.
var newScreen = func() screen {
	a := pterm.DefaultArea
	p, _ := a.Start()
	return p
}

var frames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// live redraws the steps running in an area of the terminal. The area lives
// while a step runs; once the last one is done, it is frozen and the logs of
// the steps that failed are printed below it, for they would not fit inside.
type live struct {
	mu     sync.Mutex
	area   screen
	steps  []*liveStep
	active int
	frame  int
	stop   chan struct{}
}

type liveStep struct {
	r       *live
	name    string
	detail  string
	start   time.Time
	log     buffer
	done    bool
	elapsed time.Duration
	result  Result
}

func (r *live) Start(name, detail string) Step {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := &liveStep{r: r, name: name, detail: detail, start: time.Now()}
	r.steps = append(r.steps, s)
	r.active++
	if r.active == 1 {
		r.area = newScreen()
		r.stop = make(chan struct{})
		go r.tick(r.stop)
	}
	r.render()
	return s
}

// tick animates the spinners until its area is closed. It checks stop under
// the lock: Finish closes the area holding it, without waiting for tick.
func (r *live) tick(stop chan struct{}) {
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	for range t.C {
		r.mu.Lock()
		select {
		case <-stop:
			r.mu.Unlock()
			return
		default:
		}
		r.frame++
		r.render()
		r.mu.Unlock()
	}
}

func (r *live) render() {
	var b strings.Builder
	for _, s := range r.steps {
		var icon string
		d := time.Since(s.start)
		switch {
		case !s.done:
			icon = pterm.Cyan(frames[r.frame%len(frames)])
		case s.result.Err != nil:
			icon, d = failIcon(), s.elapsed
		default:
			icon, d = okIcon(), s.elapsed
		}
		b.WriteString(line(icon, s.name, s.detail, s.result.Summary, d))
		b.WriteByte('\n')
	}
	r.area.Update(strings.TrimRight(b.String(), "\n"))
}

func (s *liveStep) Log() io.Writer { return &s.log }

func (s *liveStep) SetDetail(detail string) {
	s.r.mu.Lock()
	defer s.r.mu.Unlock()
	s.detail = detail
}

func (s *liveStep) Finish(res Result) {
	r := s.r
	r.mu.Lock()
	defer r.mu.Unlock()
	s.done, s.elapsed, s.result = true, time.Since(s.start), res
	r.active--
	r.render()
	if r.active > 0 {
		return
	}
	close(r.stop)
	_ = r.area.Stop()
	// The area leaves the cursor at the end of its last line
	fmt.Fprintln(Output)
	for _, s := range r.steps {
		details(Output, s.log.String(), s.result.Notes, s.result.Err != nil)
	}
	r.steps = nil
}
