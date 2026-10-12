package gox

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// feed gives the events to a run, as go test -json writes them.
func feed(t *testing.T, r *testRun, events ...event) {
	t.Helper()
	w := &lines{fn: r.handle}
	for _, e := range events {
		b, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(w, "%s\n", b)
	}
}

func TestTestRunCountsAndReports(t *testing.T) {
	var progress []string
	r := newTestRun(func(s string) { progress = append(progress, s) })
	const a, b, c = "example.com/m/a", "example.com/m/b", "example.com/m/c"
	feed(t, r,
		event{Action: "build-output", ImportPath: c + " [" + c + ".test]", Output: "c/c.go:3:1: syntax error\n"},
		event{Action: "build-fail", ImportPath: c + " [" + c + ".test]"},
		event{Action: "start", Package: a},
		event{Action: "run", Package: a, Test: "TestOK"},
		event{Action: "pass", Package: a, Test: "TestOK"},
		event{Action: "run", Package: a, Test: "TestSkipped"},
		event{Action: "skip", Package: a, Test: "TestSkipped"},
		event{Action: "run", Package: a, Test: "TestParent"},
		event{Action: "run", Package: a, Test: "TestParent/sub"},
		event{Action: "output", Package: a, Test: "TestParent/sub", Output: "=== RUN   TestParent/sub\n"},
		event{Action: "output", Package: a, Test: "TestParent/sub", Output: "    a_test.go:9: got 3, want 4\n"},
		event{Action: "output", Package: a, Test: "TestParent/sub", Output: "    --- FAIL: TestParent/sub (0.00s)\n"},
		event{Action: "fail", Package: a, Test: "TestParent/sub", Elapsed: 0.01},
		event{Action: "output", Package: a, Test: "TestParent", Output: "--- FAIL: TestParent (0.01s)\n"},
		event{Action: "fail", Package: a, Test: "TestParent", Elapsed: 0.01},
		event{Action: "output", Package: a, Output: "FAIL\n"},
		event{Action: "fail", Package: a},
		event{Action: "start", Package: b},
		event{Action: "output", Package: b, Output: "panic: init failed\n"},
		event{Action: "fail", Package: b},
		event{Action: "start", Package: c},
		event{Action: "output", Package: c, Output: "FAIL\t" + c + " [build failed]\n"},
		event{Action: "fail", Package: c},
	)
	fmt.Fprint(&lines{fn: r.handle}, "go: downloading example.com/dep v1.0.0\n")

	if r.passed != 1 || r.skipped != 1 || r.failed != 2 {
		t.Errorf("passed %d, skipped %d, failed %d", r.passed, r.skipped, r.failed)
	}
	if want := "1 passed · 1 skipped · 2 failed"; r.summary() != want {
		t.Errorf("summary = %q, want %q", r.summary(), want)
	}
	if !slices.Equal(r.pkgFailed, []string{a, b, c}) {
		t.Errorf("packages failed = %q", r.pkgFailed)
	}
	if want := "3 packages · 1 passed · 1 skipped · 2 failed"; progress[len(progress)-1] != want {
		t.Errorf("progress = %q, want %q at the end", progress, want)
	}

	var report strings.Builder
	r.report(&report)
	got := report.String()
	for _, want := range []string{
		"go: downloading example.com/dep v1.0.0\n",
		"--- BUILD FAILED  " + c + " [" + c + ".test]\nc/c.go:3:1: syntax error\n",
		"--- FAIL  TestParent/sub (0.01s)  " + a + "\n    a_test.go:9: got 3, want 4\n",
		"--- FAIL  " + b + "\npanic: init failed\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report lacks %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{
		"FAIL  TestParent (", // its subtest tells what failed
		"=== RUN",
		"--- FAIL: TestParent/sub",
		"--- FAIL  " + c + "\n", // its build tells it
	} {
		if strings.Contains(got, unwanted) {
			t.Errorf("report has %q:\n%s", unwanted, got)
		}
	}
}

func TestLinesSplitsAcrossWrites(t *testing.T) {
	var got []string
	w := &lines{fn: func(l []byte) { got = append(got, string(l)) }}
	fmt.Fprint(w, "one\ntw")
	fmt.Fprint(w, "o\n\nthree")
	if want := []string{"one", "two", ""}; !slices.Equal(got, want) {
		t.Errorf("lines = %q, want %q: the unfinished line waits", got, want)
	}
}

func TestDescribe(t *testing.T) {
	for _, c := range []struct {
		pkgs []string
		want string
	}{
		{[]string{"./..."}, "./..."},
		{[]string{"./a", "./b"}, "./a ./b"},
		{[]string{"./a", "./b", "./c", "./d"}, "./a +3"},
	} {
		if got := describe(c.pkgs); got != c.want {
			t.Errorf("describe(%q) = %q, want %q", c.pkgs, got, c.want)
		}
	}
	if got := orAll(nil); !slices.Equal(got, []string{"./..."}) {
		t.Errorf("orAll(nil) = %q", got)
	}
}
