// Package gox runs the Go toolchain as steps of magox: build, test,
// coverage, lint.
package gox

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/gsontag/magox"
)

// Build builds packages, for the host or for other platforms.
type Build struct {
	Pkgs []string // "." if empty
	// Out is the binary written; with several platforms, each gets
	// -os-arch appended. Empty, the packages are compiled and thrown away:
	// what checks they build.
	Out       string
	Platforms []string // "os/arch"; the host if empty
	Tags      []string
	Ldflags   string
	Trimpath  bool
	Env       []string // "KEY=value"
}

// Run builds for each platform, all at once.
func (b Build) Run() error {
	if len(b.Platforms) == 0 {
		return b.build("")
	}
	var wg sync.WaitGroup
	errs := make([]error, len(b.Platforms))
	for i, p := range b.Platforms {
		wg.Go(func() { errs[i] = b.build(p) })
	}
	wg.Wait()
	return errors.Join(errs...)
}

func (b Build) build(platform string) error {
	pkgs := b.Pkgs
	if len(pkgs) == 0 {
		pkgs = []string{"."}
	}
	args := []string{"build"}
	if b.Trimpath {
		args = append(args, "-trimpath")
	}
	if len(b.Tags) > 0 {
		args = append(args, "-tags", strings.Join(b.Tags, ","))
	}
	if b.Ldflags != "" {
		args = append(args, "-ldflags", b.Ldflags)
	}
	env := b.Env
	detail := b.Out
	if platform != "" {
		goos, goarch, ok := strings.Cut(platform, "/")
		if !ok {
			return fmt.Errorf("platform %q: want os/arch", platform)
		}
		env = append(append([]string{}, env...), "GOOS="+goos, "GOARCH="+goarch)
		detail = platform
	}
	if b.Out != "" {
		out := b.Out
		if platform != "" {
			out += "-" + strings.ReplaceAll(platform, "/", "-")
			if strings.HasPrefix(platform, "windows/") {
				out += ".exe"
			}
		}
		args = append(args, "-o", out)
		detail = out
	}
	if detail == "" {
		detail = describe(pkgs)
	}
	args = append(args, pkgs...)
	return magox.Run("build", detail, func(s *magox.Step) error {
		return s.Command("go", args...).Env(env...).Run()
	})
}

// describe tells the packages in a few words: a long list would not fit on
// the line of a step.
func describe(pkgs []string) string {
	if len(pkgs) <= 2 {
		return strings.Join(pkgs, " ")
	}
	return fmt.Sprintf("%s +%d", pkgs[0], len(pkgs)-1)
}

// orAll is ./..., the packages of the module, if pkgs is empty.
func orAll(pkgs []string) []string {
	if len(pkgs) == 0 {
		return []string{"./..."}
	}
	return pkgs
}

// Generate runs go generate on the packages, ./... if none.
func Generate(pkgs ...string) error {
	pkgs = orAll(pkgs)
	return magox.Run("generate", describe(pkgs), func(s *magox.Step) error {
		return s.Command("go", append([]string{"generate"}, pkgs...)...).Run()
	})
}
