package report

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pterm/pterm"
)

func TestMain(m *testing.M) {
	pterm.DisableColor()
	os.Exit(m.Run())
}

// capture sends Output to a buffer for the test, and resets the alignment of
// the names, that steps of other tests would widen.
func capture(t *testing.T) *syncBuffer {
	t.Helper()
	b := &syncBuffer{}
	old := Output
	Output = b
	names.width = 0
	t.Cleanup(func() { Output = old })
	return b
}

type syncBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

func TestDuration(t *testing.T) {
	for d, want := range map[time.Duration]string{
		0:                       "0.0s",
		1234 * time.Millisecond: "1.2s",
		72 * time.Second:        "1m12s",
	} {
		if got := duration(d); got != want {
			t.Errorf("duration(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestPadAlignsOnLongestName(t *testing.T) {
	names.width = 0
	if got := pad("vet"); got != "vet     " {
		t.Errorf("pad under the minimum = %q, want 8 wide", got)
	}
	pad("coverageHTML")
	if got := pad("vet"); len(got) != len("coverageHTML") {
		t.Errorf("pad after a longer name = %q, want %d wide", got, len("coverageHTML"))
	}
}

func TestPlainSuccess(t *testing.T) {
	out := capture(t)
	r := &plain{}
	s := r.Start("lint", "golangci-lint run")
	fmt.Fprintln(s.Log(), "hidden: the step succeeds")
	s.SetDetail("./...")
	s.Finish(Result{Summary: "0 issues", Notes: "a note"})

	got := out.String()
	if !strings.HasPrefix(got, "✓ lint      ./...  0 issues  ") {
		t.Errorf("line = %q", got)
	}
	if strings.Contains(got, "hidden") {
		t.Errorf("the log of a step that succeeds shows: %q", got)
	}
	if !strings.Contains(got, "\n    a note\n") {
		t.Errorf("the note is missing: %q", got)
	}
}

func TestPlainFailureShowsLog(t *testing.T) {
	out := capture(t)
	r := &plain{}
	s := r.Start("test", "./...")
	fmt.Fprint(s.Log(), "line one\nline two\n")
	s.Finish(Result{Err: errors.New("boom")})

	got := out.String()
	if !strings.HasPrefix(got, "✗ test") {
		t.Errorf("line = %q", got)
	}
	if !strings.Contains(got, "  │ line one\n  │ line two\n") {
		t.Errorf("the log is not in the gutter: %q", got)
	}
	if strings.Contains(got, "::error") {
		t.Errorf("annotation out of GitHub: %q", got)
	}
}

func TestGitHubAnnotation(t *testing.T) {
	out := capture(t)
	r := &plain{github: true}
	s := r.Start("test", "")
	s.Finish(Result{Err: errors.New("2 tests failed\n100% sure")})

	want := "::error title=test::2 tests failed%0A100%25 sure\n"
	if got := out.String(); !strings.HasSuffix(got, want) {
		t.Errorf("output = %q, want it to end with %q", got, want)
	}
}

func TestVerboseStreamsPrefixedLines(t *testing.T) {
	out := capture(t)
	r := &plain{verbose: true}
	s := r.Start("vet", "./...")
	fmt.Fprint(s.Log(), "first\nsec")
	fmt.Fprint(s.Log(), "ond\nlast, unfinished")
	s.Finish(Result{Err: errors.New("failed")})

	got := out.String()
	for _, want := range []string{"▸ vet  ./...\n", "vet │ first\n", "vet │ second\n", "vet │ last, unfinished\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	// streamed already, the log is not written again with the failure
	if strings.Count(got, "first") != 1 {
		t.Errorf("the log shows twice: %q", got)
	}
}

// fakeScreen records what the live reporter draws.
type fakeScreen struct {
	mu      sync.Mutex
	content string
	stopped bool
}

func (f *fakeScreen) Update(text ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.content = fmt.Sprint(text...)
}

func (f *fakeScreen) Stop() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped = true
	return nil
}

func (f *fakeScreen) get() (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.content, f.stopped
}

func fakeScreens(t *testing.T) *[]*fakeScreen {
	t.Helper()
	var screens []*fakeScreen
	old := newScreen
	newScreen = func() screen {
		s := &fakeScreen{}
		screens = append(screens, s)
		return s
	}
	t.Cleanup(func() { newScreen = old })
	return &screens
}

func TestLiveDrawsStepsUntilTheLastIsDone(t *testing.T) {
	out := capture(t)
	screens := fakeScreens(t)
	r := &live{}

	build := r.Start("build", "bin/app")
	test := r.Start("test", "./...")
	fmt.Fprintln(test.Log(), "--- FAIL  TestX")
	build.Finish(Result{Summary: "ok"})

	content, stopped := (*screens)[0].get()
	if stopped {
		t.Fatal("the screen stops while a step runs")
	}
	lines := strings.Split(content, "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "✓ build") || strings.HasPrefix(lines[1], "✓") {
		t.Errorf("screen = %q, want build done and test running", content)
	}
	if out.String() != "" {
		t.Errorf("details written while a step runs: %q", out.String())
	}

	test.Finish(Result{Err: errors.New("1 tests failed"), Notes: "see above"})
	content, stopped = (*screens)[0].get()
	if !stopped || !strings.Contains(content, "✗ test") {
		t.Errorf("screen = %q, stopped %v: want the failure, frozen", content, stopped)
	}
	if got := out.String(); got != "  │ --- FAIL  TestX\n    see above\n" {
		t.Errorf("details = %q", got)
	}

	// A step after opens a new screen, without the steps before
	r.Start("lint", "").Finish(Result{})
	if len(*screens) != 2 {
		t.Fatalf("%d screens, want 2", len(*screens))
	}
	if content, _ := (*screens)[1].get(); strings.Contains(content, "build") {
		t.Errorf("the second screen draws the steps of the first: %q", content)
	}
}

func TestLiveConcurrentSteps(t *testing.T) {
	capture(t)
	fakeScreens(t)
	r := &live{}
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Go(func() {
			s := r.Start(fmt.Sprintf("step%d", i), "")
			fmt.Fprintln(s.Log(), "output")
			s.SetDetail("working")
			time.Sleep(time.Millisecond)
			s.Finish(Result{})
		})
	}
	wg.Wait()
	if r.active != 0 || r.steps != nil {
		t.Errorf("active %d, %d steps left", r.active, len(r.steps))
	}
}
