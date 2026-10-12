package gox

import (
	"io"
	"regexp"
	"strings"

	"github.com/gsontag/magox"
	"github.com/gsontag/magox/internal/tool"
)

const golangci = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint"

// Lint runs golangci-lint, with the configuration of the project.
type Lint struct {
	Fix  bool
	Args []string
}

var issuesRe = regexp.MustCompile(`(?m)^(\d+) issues?[.:]`)

// Run lints.
func (l Lint) Run() error {
	args := []string{"run"}
	if l.Fix {
		args = append(args, "--fix")
	}
	return golangciStep("lint", append(args, l.Args...), func(s *magox.Step, out string) {
		if m := issuesRe.FindStringSubmatch(out); m != nil {
			s.Summary(m[1] + " issues")
		}
	})
}

// Fmt formats the code with the formatters of the configuration.
func Fmt() error { return golangciStep("fmt", []string{"fmt"}, nil) }

// FmtCheck fails if some code is not formatted, its diff in the log.
func FmtCheck() error { return golangciStep("fmt", []string{"fmt", "--diff"}, nil) }

func golangciStep(name string, args []string, after func(*magox.Step, string)) error {
	return magox.Run(name, "golangci-lint "+strings.Join(args, " "), func(s *magox.Step) error {
		cmd, err := tool.Command("golangci-lint", golangci)
		if err != nil {
			return err
		}
		var out strings.Builder
		c := s.Command(cmd[0], append(cmd[1:], args...)...)
		c.Stdout(io.MultiWriter(&out, s.Log()))
		err = c.Run()
		if after != nil {
			after(s, out.String())
		}
		return err
	})
}

// Vet runs go vet on the packages, ./... if none.
func Vet(pkgs ...string) error {
	pkgs = orAll(pkgs)
	return magox.Run("vet", describe(pkgs), func(s *magox.Step) error {
		return s.Command("go", append([]string{"vet"}, pkgs...)...).Run()
	})
}

// Tidy runs go mod tidy.
func Tidy() error {
	return magox.Run("tidy", "go.mod", func(s *magox.Step) error {
		return s.Command("go", "mod", "tidy").Run()
	})
}

// TidyCheck fails if go.mod or go.sum are not tidy, the diff in the log.
func TidyCheck() error {
	return magox.Run("tidy", "go.mod --diff", func(s *magox.Step) error {
		return s.Command("go", "mod", "tidy", "-diff").Run()
	})
}
