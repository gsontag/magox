package gox

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/pterm/pterm"

	"github.com/gsontag/magox"
)

// Coverage reports the coverage of a profile that Test wrote.
type Coverage struct {
	Profile  string  // coverage.out if empty
	Min      float64 // the step fails below this total, in percent; 0 for none
	Packages bool    // a table of the packages below the step
}

// Run reports the coverage.
func (c Coverage) Run() error {
	file := c.Profile
	if file == "" {
		file = "coverage.out"
	}
	return magox.Run("coverage", file, func(s *magox.Step) error {
		p, err := readProfile(file)
		if err != nil {
			return err
		}
		covered, total := p.total()
		pct := percent(covered, total)
		s.Summary(fmt.Sprintf("%.1f%% of %d statements", pct, total))
		if c.Packages {
			s.Note(p.table())
		}
		if c.Min > 0 && pct < c.Min {
			return fmt.Errorf("coverage %.1f%% under %.1f%%", pct, c.Min)
		}
		return nil
	})
}

// CoverHTML opens the profile in the browser.
func CoverHTML(profile string) error {
	return magox.Run("coverage", profile+" → browser", func(s *magox.Step) error {
		return s.Command("go", "tool", "cover", "-html="+profile).Run()
	})
}

type block struct {
	stmts   int
	covered bool
}

type profile struct {
	mode   string
	lines  []string          // the blocks, as written
	blocks map[string]*block // by "file:range", once each
}

func readProfile(file string) (*profile, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	p := &profile{blocks: map[string]*block{}}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := sc.Text()
		if m, ok := strings.CutPrefix(l, "mode: "); ok {
			p.mode = m
			continue
		}
		// file:12.3,14.5 numStmt count
		fields := strings.Fields(l)
		if len(fields) != 3 {
			continue
		}
		stmts, err1 := strconv.Atoi(fields[1])
		count, err2 := strconv.Atoi(fields[2])
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("%s: bad line %q", file, l)
		}
		p.lines = append(p.lines, l)
		// The packages tested together may each write the same block
		b := p.blocks[fields[0]]
		if b == nil {
			b = &block{stmts: stmts}
			p.blocks[fields[0]] = b
		}
		b.covered = b.covered || count > 0
	}
	return p, sc.Err()
}

func (p *profile) total() (covered, total int) {
	for _, b := range p.blocks {
		total += b.stmts
		if b.covered {
			covered += b.stmts
		}
	}
	return covered, total
}

func (p *profile) table() string {
	type pkg struct{ covered, total int }
	pkgs := map[string]*pkg{}
	for key, b := range p.blocks {
		file, _, _ := strings.Cut(key, ":")
		dir := path.Dir(file)
		if pkgs[dir] == nil {
			pkgs[dir] = &pkg{}
		}
		pkgs[dir].total += b.stmts
		if b.covered {
			pkgs[dir].covered += b.stmts
		}
	}
	names := make([]string, 0, len(pkgs))
	for n := range pkgs {
		names = append(names, n)
	}
	sort.Strings(names)
	prefix := commonPrefix(names)
	data := make(pterm.TableData, 0, 1+len(names))
	data = append(data, []string{"package", "coverage", "statements"})
	for _, n := range names {
		pc := percent(pkgs[n].covered, pkgs[n].total)
		name := strings.TrimPrefix(n, prefix)
		if name == "" {
			name = "."
		}
		data = append(data, []string{name, colorPercent(pc), strconv.Itoa(pkgs[n].total)})
	}
	t, _ := pterm.DefaultTable.WithHasHeader().WithData(data).Srender()
	return t
}

// commonPrefix is the module, or what the packages share: the table would
// repeat it on every line.
func commonPrefix(names []string) string {
	if len(names) < 2 {
		return ""
	}
	p := names[0]
	for _, n := range names[1:] {
		for !strings.HasPrefix(n+"/", p+"/") {
			p = path.Dir(p)
			if p == "." || p == "/" {
				return ""
			}
		}
	}
	return p + "/"
}

func colorPercent(pc float64) string {
	s := fmt.Sprintf("%.1f%%", pc)
	switch {
	case pc >= 80:
		return pterm.Green(s)
	case pc >= 50:
		return pterm.Yellow(s)
	default:
		return pterm.Red(s)
	}
}

func percent(covered, total int) float64 {
	if total == 0 {
		return 100
	}
	return 100 * float64(covered) / float64(total)
}

var generatedRe = regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.$`)

// excludeGenerated rewrites the profile without the blocks of the generated
// files.
func excludeGenerated(file string) error {
	p, err := readProfile(file)
	if err != nil {
		return err
	}
	mods, err := modules()
	if err != nil {
		return err
	}
	generated := map[string]bool{}
	var b strings.Builder
	fmt.Fprintf(&b, "mode: %s\n", p.mode)
	for _, l := range p.lines {
		src, _, _ := strings.Cut(l, ":")
		g, seen := generated[src]
		if !seen {
			g = isGenerated(localPath(mods, src))
			generated[src] = g
		}
		if !g {
			b.WriteString(l + "\n")
		}
	}
	return os.WriteFile(file, []byte(b.String()), 0o644)
}

// modules maps the paths of the modules of the build to their directory.
func modules() (map[string]string, error) {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Path}}\t{{.Dir}}").Output()
	if err != nil {
		return nil, fmt.Errorf("go list -m: %w", err)
	}
	mods := map[string]string{}
	for l := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if p, d, ok := strings.Cut(l, "\t"); ok {
			mods[p] = d
		}
	}
	return mods, nil
}

func localPath(mods map[string]string, src string) string {
	for p, d := range mods {
		if rest, ok := strings.CutPrefix(src, p+"/"); ok {
			return filepath.Join(d, rest)
		}
	}
	return src
}

// isGenerated follows the convention of Go: the comment before the package
// clause.
func isGenerated(file string) bool {
	f, err := os.Open(file)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := sc.Text()
		if strings.HasPrefix(l, "package ") {
			return false
		}
		if generatedRe.MatchString(l) {
			return true
		}
	}
	return false
}
