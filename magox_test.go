package magox

import (
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/gsontag/magox/internal/report"
)

// fakeReporter records the steps.
type fakeReporter struct {
	mu    sync.Mutex
	steps []*fakeStep
}

type fakeStep struct {
	name, detail string
	log          strings.Builder
	result       *report.Result
}

func (r *fakeReporter) Start(name, detail string) report.Step {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := &fakeStep{name: name, detail: detail}
	r.steps = append(r.steps, s)
	return s
}

func (s *fakeStep) Log() io.Writer          { return &s.log }
func (s *fakeStep) SetDetail(detail string) { s.detail = detail }
func (s *fakeStep) Finish(r report.Result)  { s.result = &r }

func fake(t *testing.T) *fakeReporter {
	t.Helper()
	f := &fakeReporter{}
	old := reporter
	reporter = func() report.Reporter { return f }
	t.Cleanup(func() { reporter = old })
	return f
}

func TestRunSuccess(t *testing.T) {
	f := fake(t)
	err := Run("build", "bin/app", func(s *Step) error {
		s.Detail("bin/app linux")
		s.Summary("2 binaries")
		s.Note("first")
		s.Note("second\n")
		s.Logf("built %d", 2)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	s := f.steps[0]
	if s.name != "build" || s.detail != "bin/app linux" {
		t.Errorf("step %q %q", s.name, s.detail)
	}
	if s.result == nil || s.result.Summary != "2 binaries" || s.result.Notes != "first\nsecond\n" {
		t.Errorf("result = %+v", s.result)
	}
	if s.log.String() != "built 2\n" {
		t.Errorf("log = %q", s.log.String())
	}
}

func TestRunErrorNamesTheStep(t *testing.T) {
	f := fake(t)
	cause := errors.New("3 tests failed")
	err := Run("test", "", func(*Step) error { return cause })
	if !errors.Is(err, cause) || err.Error() != "test: 3 tests failed" {
		t.Errorf("err = %v", err)
	}
	if !errors.Is(f.steps[0].result.Err, cause) {
		t.Errorf("the step finishes with %v, want the error of the function", f.steps[0].result.Err)
	}
}

func TestRunPanicFinishesTheStep(t *testing.T) {
	f := fake(t)
	defer func() {
		if p := recover(); p != "boom" {
			t.Errorf("recovered %v, want the panic again", p)
		}
		if r := f.steps[0].result; r == nil || r.Err == nil || r.Err.Error() != "panic: boom" {
			t.Errorf("result = %+v: the step is not finished", r)
		}
	}()
	_ = Run("build", "", func(*Step) error { panic("boom") })
}

func TestCommandLogsItsLineAndOutput(t *testing.T) {
	f := fake(t)
	dir := t.TempDir()
	err := Run("sh", "", func(s *Step) error {
		return s.Command("sh", "-c", `echo "$GREETING from $(pwd)"; echo oops >&2`).
			Env("GREETING=hello").Dir(dir).Run()
	})
	if err != nil {
		t.Fatal(err)
	}
	log := f.steps[0].log.String()
	want := "$ (cd " + dir + " && GREETING=hello sh -c "
	if !strings.HasPrefix(log, want) {
		t.Errorf("log = %q, want it to start with %q", log, want)
	}
	if !strings.Contains(log, "hello from "+dir+"\n") || !strings.Contains(log, "oops\n") {
		t.Errorf("the output is not in the log: %q", log)
	}
}

func TestCommandError(t *testing.T) {
	fake(t)
	err := Run("sh", "", func(s *Step) error {
		return s.Command("sh", "-c", "exit 3").Run()
	})
	if err == nil || err.Error() != "sh: sh: exit status 3" {
		t.Errorf("err = %v", err)
	}
}

func TestCommandOutput(t *testing.T) {
	f := fake(t)
	var got string
	err := Run("git", "", func(s *Step) error {
		var err error
		got, err = s.Command("sh", "-c", "echo '  v1.2.3  '; echo warn >&2").Output()
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "v1.2.3" {
		t.Errorf("output = %q, want trimmed", got)
	}
	if log := f.steps[0].log.String(); strings.Contains(log, "  v1.2.3  \n") || !strings.Contains(log, "warn\n") {
		t.Errorf("log = %q: want stderr only", log)
	}
}

func TestSh(t *testing.T) {
	f := fake(t)
	if err := Sh("/bin/sh", "-c", "true"); err != nil {
		t.Fatal(err)
	}
	if s := f.steps[0]; s.name != "sh" || s.detail != "-c true" {
		t.Errorf("step %q %q, want named after the command", s.name, s.detail)
	}
}
