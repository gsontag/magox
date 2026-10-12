package gox

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/gsontag/magox"
	"github.com/gsontag/magox/internal/tool"
)

// Fix applies the fixes of go fix: the modernizers, Go of before written
// with the features of now.
func Fix(pkgs ...string) error {
	pkgs = orAll(pkgs)
	return magox.Run("fix", describe(pkgs), func(s *magox.Step) error {
		return s.Command("go", append([]string{"fix"}, pkgs...)...).Run()
	})
}

// FixCheck fails if go fix would change a file, its diff in the log. Its
// results are cached as those of go vet: it is fast once run.
func FixCheck(pkgs ...string) error {
	pkgs = orAll(pkgs)
	return magox.Run("fix", describe(pkgs)+" --diff", func(s *magox.Step) error {
		var out strings.Builder
		err := s.Command("go", append([]string{"fix", "-diff"}, pkgs...)...).Stdout(&out).Run()
		diff := out.String()
		fmt.Fprint(s.Log(), diff)
		n := 0
		for l := range strings.SplitSeq(diff, "\n") {
			if strings.HasPrefix(l, "+++ ") {
				n++
			}
		}
		if n > 0 {
			s.Summary(fmt.Sprintf("%d files to fix", n))
		}
		return err
	})
}

const goplsPkg = "golang.org/x/tools/gopls"

// Hints runs the checks of gopls down to its hints, those that go fix has
// not: the type arguments that can be inferred... gopls loads the whole
// module on each run: some seconds, a target to run apart.
type Hints struct {
	Files []string // the Go files that git follows if empty
	// Ignore drops the hints that contain one of these: "[darwin]" for those
	// of another platform
	Ignore []string
}

// Run fails if gopls gives a hint.
func (h Hints) Run() error {
	return magox.Run("hints", "gopls check", func(s *magox.Step) error {
		cmd, err := tool.Command("gopls", goplsPkg)
		if err != nil {
			return err
		}
		files := h.Files
		if len(files) == 0 {
			ls, err := s.Command("git", "ls-files", "*.go").Output()
			if err != nil {
				return err
			}
			files = strings.Fields(ls)
		}
		s.Detail(fmt.Sprintf("gopls check, %d files", len(files)))

		var out strings.Builder
		args := append(append(cmd[1:], "check", "-severity=hint"), files...)
		c := s.Command(cmd[0], args...).Stdout(&out).Stderr(&out)
		// gopls exits with 0 whatever it finds: its output tells
		if err := c.Run(); err != nil && !isExit(err) {
			return err
		}
		hints := filterHints(out.String(), h.Ignore)
		if len(hints) == 0 {
			return nil
		}
		fmt.Fprintln(s.Log(), strings.Join(hints, "\n"))
		s.Summary(fmt.Sprintf("%d hints", len(hints)))
		return fmt.Errorf("%d hints", len(hints))
	})
}

func isExit(err error) bool {
	var e *exec.ExitError
	return errors.As(err, &e)
}

// filterHints keeps the lines of out that contain none of ignore.
func filterHints(out string, ignore []string) []string {
	var hints []string
lines:
	for l := range strings.SplitSeq(out, "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		for _, i := range ignore {
			if strings.Contains(l, i) {
				continue lines
			}
		}
		hints = append(hints, l)
	}
	return hints
}
