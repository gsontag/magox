package gox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/pterm/pterm"

	"github.com/gsontag/magox"
	"github.com/gsontag/magox/internal/tool"
)

const fieldalignmentPkg = "golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment"

// FieldAlignment finds the structs whose fields, in another order, would take
// less memory, through fieldalignment -json: a table of them, the most
// wasteful first.
//
// By default, it keeps only the structs that would spare memory indeed: a
// smaller size in the same size class of the allocator spares nothing.
type FieldAlignment struct {
	Pkgs  []string // ./... if empty
	Tests bool     // the structs of the tests too
	// All keeps the structs whose order spares no memory too: a smaller size
	// in the same size class
	All bool
	// Pointers keeps the structs whose pointers could come first too, that
	// the garbage collector scans less: many, for little
	Pointers bool
	// Fix rearranges the fields of the structs found. fieldalignment drops
	// the comments of the fields: the structs that have some are left, their
	// order in the log
	Fix bool
}

// alignment is a struct that fieldalignment finds.
type alignment struct {
	pos     string // file:line, relative
	name    string
	pointer bool // of the pointer bytes, not of the size
	size    int  // or the pointer bytes
	optimal int
	waste   int // the memory spared, with the size classes of the allocator
	percent int
	message string
	fix     *edit
}

type edit struct {
	file       string
	start, end int
	text       string
}

var (
	sizeRe = regexp.MustCompile(
		`^(\S+) has size (\d+)(?: \(allocator size class \d+\))? but the optimal size is (\d+)` +
			`(?: \(allocator size class \d+\))?(?: leading to a waste of (\d+) bytes \((\d+)%\))?`,
	)
	pointerRe = regexp.MustCompile(`^(\S+) has (\d+) leading bytes of pointer data but optimal value is (\d+)`)
)

// Run fails if it finds a struct to rearrange, or rearranges them with Fix.
func (f FieldAlignment) Run() error {
	pkgs := orAll(f.Pkgs)
	detail := describe(pkgs)
	if f.Fix {
		detail += " --fix"
	}
	return magox.Run("align", detail, func(s *magox.Step) error {
		cmd, err := tool.Command("fieldalignment", fieldalignmentPkg)
		if err != nil {
			return err
		}
		var out bytes.Buffer
		args := append(append(cmd[1:], "-json", "-test="+strconv.FormatBool(f.Tests)), pkgs...)
		if err := s.Command(cmd[0], args...).Stdout(&out).Run(); err != nil {
			return err
		}
		found, err := parseAlignments(out.Bytes())
		if err != nil {
			return err
		}
		found = slices.DeleteFunc(found, func(a alignment) bool {
			return (a.pointer && !f.Pointers) || (!a.pointer && a.waste == 0 && !f.All)
		})
		if len(found) == 0 {
			return nil
		}
		s.Note(alignmentTable(found))
		if f.Fix {
			return f.fix(s, found)
		}
		s.Summary(alignmentSummary(found))
		return fmt.Errorf("%d structs to rearrange", len(found))
	})
}

func (f FieldAlignment) fix(s *magox.Step, found []alignment) error {
	byFile := map[string][]alignment{}
	for _, a := range found {
		if a.fix != nil {
			byFile[a.fix.file] = append(byFile[a.fix.file], a)
		}
	}
	fixed, left := 0, 0
	for file, as := range byFile {
		src, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		// From the end, that the offsets of the edits left stay right
		slices.SortFunc(as, func(a, b alignment) int { return b.fix.start - a.fix.start })
		for _, a := range as {
			old := string(src[a.fix.start:a.fix.end])
			if strings.Contains(old, "//") || strings.Contains(old, "/*") {
				s.Logf("%s: %s has comments, to rearrange by hand:\n%s", a.pos, a.name, a.fix.text)
				left++
				continue
			}
			src = slices.Concat(src[:a.fix.start], []byte(a.fix.text), src[a.fix.end:])
			fixed++
		}
		if src, err = format.Source(src); err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
		if err := os.WriteFile(file, src, 0o644); err != nil {
			return err
		}
	}
	if left == 0 {
		s.Summary(fmt.Sprintf("%d structs rearranged", fixed))
		return nil
	}
	s.Summary(fmt.Sprintf("%d structs rearranged · %d by hand", fixed, left))
	return fmt.Errorf("%d structs with comments to rearrange by hand", left)
}

// parseAlignments reads the output of fieldalignment -json: by package, by
// analyzer, the diagnostics, or the error of the package.
func parseAlignments(out []byte) ([]alignment, error) {
	var pkgs map[string]map[string]json.RawMessage
	if err := json.Unmarshal(out, &pkgs); err != nil {
		return nil, fmt.Errorf("reading fieldalignment -json: %w", err)
	}
	wd, _ := os.Getwd()
	var found []alignment
	for pkg, analyzers := range pkgs {
		for _, raw := range analyzers {
			var diags []struct {
				Posn           string
				Message        string
				SuggestedFixes []struct {
					Edits []struct {
						Filename   string
						Start, End int
						New        string
					}
				} `json:"suggested_fixes"`
			}
			if err := json.Unmarshal(raw, &diags); err != nil {
				var e struct{ Error string }
				if json.Unmarshal(raw, &e) == nil && e.Error != "" {
					return nil, fmt.Errorf("%s: %s", pkg, e.Error)
				}
				return nil, fmt.Errorf("reading fieldalignment -json: %w", err)
			}
			for _, d := range diags {
				a := newAlignment(relative(wd, d.Posn), d.Message)
				if len(d.SuggestedFixes) > 0 && len(d.SuggestedFixes[0].Edits) == 1 {
					e := d.SuggestedFixes[0].Edits[0]
					a.fix = &edit{file: e.Filename, start: e.Start, end: e.End, text: e.New}
				}
				found = append(found, a)
			}
		}
	}
	slices.SortFunc(found, func(a, b alignment) int {
		if a.waste != b.waste {
			return b.waste - a.waste
		}
		return strings.Compare(a.pos, b.pos)
	})
	return found, nil
}

func newAlignment(pos, msg string) alignment {
	a := alignment{pos: pos, message: msg, name: strings.Fields(msg + " ?")[0]}
	if m := sizeRe.FindStringSubmatch(msg); m != nil {
		a.size, _ = strconv.Atoi(m[2])
		a.optimal, _ = strconv.Atoi(m[3])
		a.waste, _ = strconv.Atoi(m[4])
		a.percent, _ = strconv.Atoi(m[5])
	} else if m := pointerRe.FindStringSubmatch(msg); m != nil {
		a.pointer = true
		a.size, _ = strconv.Atoi(m[2])
		a.optimal, _ = strconv.Atoi(m[3])
	}
	return a
}

// relative drops the directory of the module from a position, file:line:col,
// and the column.
func relative(wd, posn string) string {
	if rel, err := filepath.Rel(wd, posn); err == nil && !strings.HasPrefix(rel, "..") {
		posn = rel
	}
	if i := strings.LastIndexByte(posn, ':'); i > 0 {
		posn = posn[:i]
	}
	return posn
}

func alignmentTable(found []alignment) string {
	data := make(pterm.TableData, 0, 1+len(found))
	data = append(data, []string{"struct", "position", "bytes", "optimal", "spared"})
	for _, a := range found {
		spared := "-"
		switch {
		case a.pointer:
			spared = "pointers"
		case a.waste > 0:
			spared = pterm.Yellow(fmt.Sprintf("%d (%d%%)", a.waste, a.percent))
		}
		data = append(data, []string{a.name, a.pos, strconv.Itoa(a.size), strconv.Itoa(a.optimal), spared})
	}
	t, _ := pterm.DefaultTable.WithHasHeader().WithData(data).Srender()
	return t
}

func alignmentSummary(found []alignment) string {
	waste := 0
	for _, a := range found {
		waste += a.waste
	}
	s := fmt.Sprintf("%d structs", len(found))
	if waste > 0 {
		s += fmt.Sprintf(" · %d bytes spared", waste)
	}
	return s
}
