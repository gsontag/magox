// Package magox is a layer over mage: the targets of a magefile call steps,
// shown as one line each with their state and duration, whose output shows
// only if they fail.
//
// The rendering follows the environment: live in a terminal, plain lines in
// a pipe or a CI, with annotations on GitHub Actions, the whole output with
// mage -v. MAGOX_OUTPUT=live|plain|verbose|github forces one.
package magox

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gsontag/magox/internal/report"
)

var reporter = sync.OnceValue(report.New)

// Step is a step running, given to the function of Run.
type Step struct {
	h       report.Step
	mu      sync.Mutex
	summary string
	notes   strings.Builder
}

// Run runs fn as a step named name; detail tells what it works on.
func Run(name, detail string, fn func(s *Step) error) (err error) {
	s := &Step{h: reporter().Start(name, detail)}
	defer func() {
		if p := recover(); p != nil {
			s.h.Finish(report.Result{Err: fmt.Errorf("panic: %v", p)})
			panic(p)
		}
		s.mu.Lock()
		s.h.Finish(report.Result{Summary: s.summary, Notes: s.notes.String(), Err: err})
		s.mu.Unlock()
		if err != nil {
			err = fmt.Errorf("%s: %w", name, err)
		}
	}()
	return fn(s)
}

// Sh runs a command as a step of its own.
func Sh(cmd string, args ...string) error {
	return Run(filepath.Base(cmd), strings.Join(args, " "), func(s *Step) error {
		return s.Command(cmd, args...).Run()
	})
}

// Log receives the output kept for the case the step fails.
func (s *Step) Log() io.Writer { return s.h.Log() }

// Logf writes a line to the log of the step.
func (s *Step) Logf(format string, args ...any) {
	fmt.Fprintf(s.h.Log(), format+"\n", args...)
}

// Detail replaces what the line of the step tells it works on.
func (s *Step) Detail(detail string) { s.h.SetDetail(detail) }

// Summary sets the result shown on the line of the step: "42 passed".
func (s *Step) Summary(summary string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.summary = summary
}

// Note adds text shown below the step, even if it succeeds: a report.
func (s *Step) Note(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notes.WriteString(text)
	if !strings.HasSuffix(text, "\n") {
		s.notes.WriteByte('\n')
	}
}

// Cmd is a command whose output goes to the log of its step.
type Cmd struct {
	cmd *exec.Cmd
	log io.Writer
	env []string // added to the environment, to tell them in the log
}

// Command prepares a command of the step.
func (s *Step) Command(name string, args ...string) *Cmd {
	c := exec.Command(name, args...)
	c.Stdout, c.Stderr = s.Log(), s.Log()
	return &Cmd{cmd: c, log: s.Log()}
}

// Env adds variables, "KEY=value", to the environment of the command.
func (c *Cmd) Env(kv ...string) *Cmd {
	if c.cmd.Env == nil {
		c.cmd.Env = os.Environ()
	}
	c.cmd.Env = append(c.cmd.Env, kv...)
	c.env = append(c.env, kv...)
	return c
}

// Dir sets the directory the command runs in.
func (c *Cmd) Dir(dir string) *Cmd {
	c.cmd.Dir = dir
	return c
}

// Stdout sends the standard output of the command to w instead of the log.
func (c *Cmd) Stdout(w io.Writer) *Cmd {
	c.cmd.Stdout = w
	return c
}

// Stderr sends the error output of the command to w instead of the log.
func (c *Cmd) Stderr(w io.Writer) *Cmd {
	c.cmd.Stderr = w
	return c
}

// Run runs the command, written first in the log: on a failure, it shows
// what to run again by hand.
func (c *Cmd) Run() error {
	line := strings.Join(append(append([]string{}, c.env...), c.cmd.Args...), " ")
	if c.cmd.Dir != "" {
		line = "(cd " + c.cmd.Dir + " && " + line + ")"
	}
	fmt.Fprintln(c.log, "$ "+line)
	if err := c.cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(c.cmd.Args[0]), err)
	}
	return nil
}

// Output runs the command and returns its standard output, trimmed.
func (c *Cmd) Output() (string, error) {
	var b strings.Builder
	c.cmd.Stdout = &b
	err := c.Run()
	return strings.TrimSpace(b.String()), err
}
