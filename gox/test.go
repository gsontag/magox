package gox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gsontag/magox"
)

// Test runs the tests through go test -json: the line of the step counts
// them as they run, and only the tests that fail show their output.
type Test struct {
	Pkgs    []string // ./... if empty
	Race    bool
	Short   bool
	Match   string // -run
	Count   int    // -count; 0 leaves the cache on
	Timeout time.Duration
	Tags    []string
	// Cover writes the coverage profile there, and gives the total coverage
	// on the line of the step
	Cover string
	// ExcludeGenerated takes the generated files out of the profile: what
	// tests them is their generator
	ExcludeGenerated bool
	Args             []string // more flags of go test
}

// Run runs the tests.
func (t Test) Run() error {
	pkgs := orAll(t.Pkgs)
	args := []string{"test", "-json"}
	if t.Race {
		args = append(args, "-race")
	}
	if t.Short {
		args = append(args, "-short")
	}
	if t.Match != "" {
		args = append(args, "-run", t.Match)
	}
	if t.Count > 0 {
		args = append(args, "-count", strconv.Itoa(t.Count))
	}
	if t.Timeout > 0 {
		args = append(args, "-timeout", t.Timeout.String())
	}
	if len(t.Tags) > 0 {
		args = append(args, "-tags", strings.Join(t.Tags, ","))
	}
	if t.Cover != "" {
		args = append(args, "-coverprofile", t.Cover)
	}
	args = append(append(args, t.Args...), pkgs...)

	return magox.Run("test", describe(pkgs), func(s *magox.Step) error {
		run := newTestRun(s)
		err := s.Command("go", args...).Stdout(&lines{fn: run.handle}).Run()
		run.report(s.Log())
		s.Detail(describe(pkgs))
		summary := fmt.Sprintf("%d packages · %s", run.pkgs, run.summary())
		defer func() { s.Summary(summary) }()
		// go test writes the profile even if tests fail
		if t.Cover != "" {
			if _, statErr := os.Stat(t.Cover); statErr == nil {
				pct, coverErr := t.coverage()
				if coverErr != nil {
					return coverErr
				}
				summary += fmt.Sprintf(" · coverage %.1f%%", pct)
			}
		}
		switch {
		case run.failed > 0:
			return fmt.Errorf("%d tests failed", run.failed)
		case len(run.pkgFailed) > 0:
			return fmt.Errorf("%d packages failed", len(run.pkgFailed))
		}
		return err
	})
}

func (t Test) coverage() (float64, error) {
	if t.ExcludeGenerated {
		if err := excludeGenerated(t.Cover); err != nil {
			return 0, err
		}
	}
	p, err := readProfile(t.Cover)
	if err != nil {
		return 0, err
	}
	return percent(p.total()), nil
}

const (
	actPass = "pass"
	actFail = "fail"
	actSkip = "skip"
)

type event struct {
	Action     string
	Package    string
	ImportPath string // of the build events
	Test       string
	Output     string
	Elapsed    float64
}

type testCase struct {
	pkg, name string
	output    []string
	elapsed   float64
}

type testRun struct {
	step                    *magox.Step
	passed, failed, skipped int
	pkgs                    int
	pkgOutput               map[string][]string
	pkgFailed               []string
	tests                   map[string]*testCase
	failedTests             []*testCase
	buildOutput             map[string][]string
	buildFailed             []string
	other                   bytes.Buffer // what go test wrote that is not JSON
}

func newTestRun(s *magox.Step) *testRun {
	return &testRun{
		step:        s,
		pkgOutput:   map[string][]string{},
		tests:       map[string]*testCase{},
		buildOutput: map[string][]string{},
	}
}

func (r *testRun) handle(l []byte) {
	var e event
	if len(l) == 0 || l[0] != '{' || json.Unmarshal(l, &e) != nil {
		r.other.Write(l)
		r.other.WriteByte('\n')
		return
	}
	key := e.Package + " " + e.Test
	switch e.Action {
	case "build-output":
		r.buildOutput[e.ImportPath] = append(r.buildOutput[e.ImportPath], e.Output)
	case "build-fail":
		r.buildFailed = append(r.buildFailed, e.ImportPath)
	case "start":
		r.pkgs++
	case "run":
		r.tests[key] = &testCase{pkg: e.Package, name: e.Test}
	case "output":
		if e.Test == "" {
			r.pkgOutput[e.Package] = append(r.pkgOutput[e.Package], e.Output)
		} else if tc := r.tests[key]; tc != nil {
			tc.output = append(tc.output, e.Output)
		}
	case actPass, actFail, actSkip:
		if e.Test == "" {
			if e.Action == actFail {
				r.pkgFailed = append(r.pkgFailed, e.Package)
			}
			r.step.Detail(fmt.Sprintf("%d packages · %s", r.pkgs, r.summary()))
			return
		}
		switch e.Action {
		case actPass:
			r.passed++
		case actSkip:
			r.skipped++
		case actFail:
			r.failed++
			if tc := r.tests[key]; tc != nil {
				tc.elapsed = e.Elapsed
				r.failedTests = append(r.failedTests, tc)
			}
		}
	}
}

func (r *testRun) summary() string {
	s := fmt.Sprintf("%d passed", r.passed)
	if r.skipped > 0 {
		s += fmt.Sprintf(" · %d skipped", r.skipped)
	}
	if r.failed > 0 {
		s += fmt.Sprintf(" · %d failed", r.failed)
	}
	return s
}

// report writes what failed: the builds, the tests, the packages failed
// out of any test (a panic in init, TestMain).
func (r *testRun) report(w io.Writer) {
	_, _ = w.Write(r.other.Bytes())
	for _, pkg := range r.buildFailed {
		fmt.Fprintf(w, "--- BUILD FAILED  %s\n", pkg)
		fmt.Fprint(w, strings.Join(r.buildOutput[pkg], ""))
	}
	pkgWithTests := map[string]bool{}
	for _, tc := range r.failedTests {
		pkgWithTests[tc.pkg] = true
		if r.hasFailedSubtest(tc) {
			continue // its subtests tell what failed
		}
		fmt.Fprintf(w, "--- FAIL  %s (%.2fs)  %s\n", tc.name, tc.elapsed, tc.pkg)
		for _, l := range tc.output {
			if !noise(l) {
				fmt.Fprint(w, l)
			}
		}
	}
	for _, pkg := range r.pkgFailed {
		if pkgWithTests[pkg] || contains(r.buildFailed, pkg) {
			continue
		}
		fmt.Fprintf(w, "--- FAIL  %s\n", pkg)
		fmt.Fprint(w, strings.Join(r.pkgOutput[pkg], ""))
	}
}

func (r *testRun) hasFailedSubtest(tc *testCase) bool {
	for _, o := range r.failedTests {
		if o.pkg == tc.pkg && strings.HasPrefix(o.name, tc.name+"/") {
			return true
		}
	}
	return false
}

// noise tells the lines that go test writes about the running of a test,
// already on the line of the test that failed.
func noise(l string) bool {
	t := strings.TrimSpace(l)
	for _, p := range []string{"=== RUN", "=== PAUSE", "=== CONT", "=== NAME", "--- FAIL:", "--- PASS:", "--- SKIP:"} {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return false
}

func contains(list []string, s string) bool {
	for _, x := range list {
		// a build event names the package, then the test binary: "pkg [pkg.test]"
		if x == s || strings.HasPrefix(x, s+" ") {
			return true
		}
	}
	return false
}

// lines calls fn on each line written to it.
type lines struct {
	fn   func([]byte)
	rest []byte
}

func (w *lines) Write(p []byte) (int, error) {
	w.rest = append(w.rest, p...)
	for {
		i := bytes.IndexByte(w.rest, '\n')
		if i < 0 {
			return len(p), nil
		}
		w.fn(w.rest[:i])
		w.rest = w.rest[i+1:]
	}
}
