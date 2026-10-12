// Package report renders the steps of a build: live in a terminal, as plain
// lines elsewhere, with the annotations of GitHub Actions on its runners.
package report

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/magefile/mage/mg"
	"github.com/pterm/pterm"
	"golang.org/x/term"
)

// Result is what a step leaves once done.
type Result struct {
	Summary string // a few words on the line of the step
	Notes   string // shown below the step, failed or not
	Err     error
}

// Reporter starts the steps; several may run at once, as mg.Deps does.
type Reporter interface {
	Start(name, detail string) Step
}

// Step is one step running.
type Step interface {
	// Log receives the output of the step: it shows only if the step fails,
	// or as it comes in verbose mode
	Log() io.Writer
	SetDetail(detail string)
	Finish(Result)
}

const modeGitHub = "github"

// Output is where the steps are written; the tests replace it.
var Output io.Writer = os.Stdout

// New chooses the rendering from MAGOX_OUTPUT (live, plain, verbose, github),
// else from the environment.
func New() Reporter {
	mode := os.Getenv("MAGOX_OUTPUT")
	tty := term.IsTerminal(int(os.Stdout.Fd()))
	if mode == "" {
		switch {
		case os.Getenv("GITHUB_ACTIONS") == "true":
			mode = modeGitHub
		case mg.Verbose():
			mode = "verbose"
		case tty:
			mode = "live"
		default:
			mode = "plain"
		}
	}
	// GitHub renders the colors of its logs, a file or a pipe does not
	if (!tty && mode != modeGitHub) || os.Getenv("NO_COLOR") != "" {
		pterm.DisableColor()
	}
	switch mode {
	case "live":
		return &live{}
	case "verbose":
		return &plain{verbose: true}
	case modeGitHub:
		return &plain{github: true}
	default:
		return &plain{}
	}
}

// names aligns the names of the steps on the longest seen so far.
var names struct {
	sync.Mutex
	width int
}

func pad(name string) string {
	names.Lock()
	defer names.Unlock()
	names.width = max(names.width, len(name), 8)
	return name + strings.Repeat(" ", names.width-len(name))
}

func line(icon, name, detail, summary string, d time.Duration) string {
	s := icon + " " + pterm.Bold.Sprint(pad(name))
	if detail != "" {
		s += "  " + detail
	}
	if summary != "" {
		s += "  " + pterm.Cyan(summary)
	}
	return s + "  " + pterm.Gray(duration(d))
}

func duration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return d.Round(time.Second).String()
}

func okIcon() string   { return pterm.Green("✓") }
func failIcon() string { return pterm.Red("✗") }

// details writes the notes of a step, and its log if it failed, in a gutter.
func details(w io.Writer, log, notes string, failed bool) {
	if failed {
		if log = strings.TrimRight(log, "\n"); log != "" {
			for l := range strings.SplitSeq(log, "\n") {
				fmt.Fprintln(w, pterm.Red("  │ ")+l)
			}
		}
	}
	if notes = strings.TrimRight(notes, "\n"); notes != "" {
		for l := range strings.SplitSeq(notes, "\n") {
			fmt.Fprintln(w, "    "+l)
		}
	}
}

// buffer is a bytes.Buffer that the commands of a step may share.
type buffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (b *buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *buffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}
