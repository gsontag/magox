package gox

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

const alignJSON = `{
	"example.com/m": {
		"fieldalignment": [
			{
				"posn": "/src/m/a.go:3:13",
				"message": "Wide has size 24 (allocator size class 24) but the optimal size is 16 ` +
	`(allocator size class 16) leading to a waste of 8 bytes (50%)",
				"suggested_fixes": [{"edits": [
					{"filename": "/src/m/a.go", "start": 10, "end": 40, "new": "struct {\n\ta int64\n}"}
				]}]
			},
			{
				"posn": "/src/m/b.go:7:13",
				"message": "Same has size 40 (allocator size class 48) but the optimal size is 36 (allocator size class 48)"
			},
			{
				"posn": "/src/m/c.go:9:13",
				"message": "Ptrs has 32 leading bytes of pointer data but optimal value is 24"
			}
		]
	}
}`

func TestParseAlignments(t *testing.T) {
	t.Chdir(t.TempDir())
	found, err := parseAlignments([]byte(alignJSON))
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 3 {
		t.Fatalf("%d structs, want 3", len(found))
	}
	wide := found[0] // the most wasteful first
	if wide.name != "Wide" || wide.size != 24 || wide.optimal != 16 || wide.waste != 8 || wide.percent != 50 {
		t.Errorf("Wide = %+v", wide)
	}
	if wide.pos != "/src/m/a.go:3" {
		t.Errorf("position = %q, want without the column", wide.pos)
	}
	if wide.fix == nil || wide.fix.start != 10 || wide.fix.end != 40 {
		t.Errorf("fix = %+v", wide.fix)
	}
	for _, a := range found[1:] {
		switch a.name {
		case "Same":
			if a.pointer || a.waste != 0 || a.size != 40 || a.optimal != 36 {
				t.Errorf("Same = %+v: a smaller size, the same class, spares nothing", a)
			}
		case "Ptrs":
			if !a.pointer || a.size != 32 || a.optimal != 24 {
				t.Errorf("Ptrs = %+v", a)
			}
		default:
			t.Errorf("unexpected %+v", a)
		}
	}
}

func TestParseAlignmentsPackageError(t *testing.T) {
	_, err := parseAlignments([]byte(`{"example.com/m": {"fieldalignment": {"error": "undefined: x"}}}`))
	if err == nil || err.Error() != "example.com/m: undefined: x" {
		t.Errorf("err = %v", err)
	}
}

const unaligned = `package m

// Plain spares 8 bytes in another order.
type Plain struct {
	a bool
	b int64
	c bool
}

// Commented spares as much, but fieldalignment would drop its comments.
type Commented struct {
	a bool // first
	b int64
	c bool
}
`

func TestFieldAlignmentFix(t *testing.T) {
	if testing.Short() {
		t.Skip("runs fieldalignment")
	}
	if _, err := exec.LookPath("fieldalignment"); err != nil {
		t.Skip("fieldalignment is not on the PATH")
	}
	module(t, map[string]string{
		"go.mod": "module example.com/m\n\ngo 1.24\n",
		"m.go":   unaligned,
	})
	err := FieldAlignment{}.Run()
	if err == nil || err.Error() != "align: 2 structs to rearrange" {
		t.Errorf("err = %v", err)
	}
	if got := out.take(); !strings.Contains(got, "Plain") || !strings.Contains(got, "8 (33%)") {
		t.Errorf("output = %q", got)
	}

	err = FieldAlignment{Fix: true}.Run()
	if err == nil || !strings.Contains(err.Error(), "1 structs with comments") {
		t.Errorf("err = %v", err)
	}
	got := out.take()
	if !strings.Contains(got, "1 structs rearranged · 1 by hand") || !strings.Contains(got, "Commented has comments") {
		t.Errorf("output = %q", got)
	}
	b, err := os.ReadFile("m.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	if !strings.Contains(src, "type Plain struct {\n\tb int64\n\ta bool\n\tc bool\n}") {
		t.Errorf("Plain not rearranged:\n%s", src)
	}
	if !strings.Contains(src, "\ta bool // first\n") {
		t.Errorf("the comment of Commented is lost:\n%s", src)
	}
}
