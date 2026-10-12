package gox

import (
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// old is Go of before, that go fix and gopls modernize.
const old = `package m

func Id[T any](v T) T { return v }

func F() int {
	n := 0
	for i := 0; i < 3; i++ {
		n += i
	}
	return Id[int](n)
}
`

func TestFixCheckThenFix(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go fix")
	}
	module(t, map[string]string{
		"go.mod": "module example.com/m\n\ngo 1.24\n",
		"m.go":   old,
	})
	if err := FixCheck(); err == nil {
		t.Error("FixCheck passes on a loop to modernize")
	}
	got := out.take()
	for _, want := range []string{"✗ fix", "1 files to fix", "│ +\tfor i := range 3 {"} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}

	if err := Fix(); err != nil {
		t.Fatalf("Fix: %v\n%s", err, out.take())
	}
	b, err := os.ReadFile("m.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "for i := range 3 {") {
		t.Errorf("m.go not fixed:\n%s", b)
	}
	if err := FixCheck(); err != nil {
		t.Errorf("FixCheck after Fix: %v\n%s", err, out.take())
	}
}

func TestHints(t *testing.T) {
	if testing.Short() {
		t.Skip("runs gopls")
	}
	if _, err := exec.LookPath("gopls"); err != nil {
		t.Skip("gopls is not on the PATH")
	}
	module(t, map[string]string{
		"go.mod": "module example.com/m\n\ngo 1.24\n",
		"m.go":   old,
	})
	err := Hints{Files: []string{"m.go"}, Ignore: []string{"modernized"}}.Run()
	if err == nil || err.Error() != "hints: 1 hints" {
		t.Errorf("err = %v, want the type arguments only", err)
	}
	got := out.take()
	if !strings.Contains(got, "unnecessary type arguments") || strings.Contains(got, "modernized") {
		t.Errorf("output = %q", got)
	}
}

func TestFilterHints(t *testing.T) {
	out := "a.go:1:1: unnecessary type arguments\n\nb.go:2:1: unused [darwin]\n  \n"
	got := filterHints(out, []string{"[darwin]"})
	if want := []string{"a.go:1:1: unnecessary type arguments"}; !slices.Equal(got, want) {
		t.Errorf("filterHints = %q, want %q", got, want)
	}
}
