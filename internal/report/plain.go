package report

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/pterm/pterm"
)

// plain writes a line per step once done, for the logs of the CI and the
// pipes. In verbose mode, the output of the steps streams as it comes.
type plain struct {
	mu      sync.Mutex
	verbose bool
	github  bool
}

type plainStep struct {
	r      *plain
	name   string
	detail string
	start  time.Time
	log    io.Writer
	buf    buffer
}

func (r *plain) Start(name, detail string) Step {
	s := &plainStep{r: r, name: name, detail: detail, start: time.Now()}
	s.log = &s.buf
	if r.verbose {
		s.log = &prefixWriter{r: r, prefix: pterm.Gray(name + " │ ")}
		r.print(pterm.Cyan("▸") + " " + pterm.Bold.Sprint(name) + "  " + detail + "\n")
	}
	return s
}

func (r *plain) print(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fmt.Fprint(Output, s)
}

func (s *plainStep) Log() io.Writer { return s.log }

func (s *plainStep) SetDetail(detail string) {
	s.r.mu.Lock()
	defer s.r.mu.Unlock()
	s.detail = detail
}

func (s *plainStep) Finish(res Result) {
	if w, ok := s.log.(*prefixWriter); ok {
		w.flush()
	}
	var b strings.Builder
	icon := okIcon()
	if res.Err != nil {
		icon = failIcon()
	}
	s.r.mu.Lock()
	detail := s.detail
	s.r.mu.Unlock()
	b.WriteString(line(icon, s.name, detail, res.Summary, time.Since(s.start)))
	b.WriteByte('\n')
	details(&b, s.buf.String(), res.Notes, res.Err != nil)
	if res.Err != nil && s.r.github {
		// An annotation shows on the summary of the run, without opening the logs
		msg := strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(res.Err.Error())
		fmt.Fprintf(&b, "::error title=%s::%s\n", s.name, msg)
	}
	s.r.print(b.String())
}

// prefixWriter writes whole lines, prefixed, that steps running together do
// not mix their lines.
type prefixWriter struct {
	r      *plain
	prefix string
	mu     sync.Mutex
	rest   []byte
}

func (w *prefixWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.rest = append(w.rest, p...)
	var b strings.Builder
	for {
		i := bytes.IndexByte(w.rest, '\n')
		if i < 0 {
			break
		}
		b.WriteString(w.prefix)
		b.Write(w.rest[:i+1])
		w.rest = w.rest[i+1:]
	}
	if b.Len() > 0 {
		w.r.print(b.String())
	}
	return len(p), nil
}

// flush writes the last line, if it has no newline.
func (w *prefixWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.rest) > 0 {
		w.r.print(w.prefix + string(w.rest) + "\n")
		w.rest = nil
	}
}
