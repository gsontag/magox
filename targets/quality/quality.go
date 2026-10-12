// Package quality gives the targets that check the code, to import in a
// magefile under a namespace:
//
//	import (
//		//mage:import quality
//		"github.com/gsontag/magox/targets/quality"
//	)
//
// They are then quality:check, quality:lint... The magefile adds its own
// checks to the namespace, and to quality:check through Config.Checks:
//
//	type Quality mg.Namespace
//
//	// Chars fails on the invisible characters.
//	func (Quality) Chars() error { ... }
//
//	func init() { quality.Config.Checks = []any{Quality.Chars} }
//
// Mage makes a target of each exported function here: the package has no
// other.
package quality

import (
	"github.com/magefile/mage/mg"

	"github.com/gsontag/magox/gox"
)

// Settings tells the targets what the project is. The magefile changes
// Config in its init: mage runs the targets after.
type Settings struct {
	Pkgs []string // the packages vetted and fixed; ./... if empty

	// HintsIgnore drops the hints of gopls that contain one of these:
	// "[darwin]" for those of another platform
	HintsIgnore []string

	// Checks are the checks of the project that Check runs with its own:
	// functions as mg.Deps takes them
	Checks []any
}

// Config is what the targets run with.
var Config Settings

// Check runs the checks, without changing a file: format, go.mod, vet,
// go fix, lint, and those of Config.Checks.
func Check() {
	mg.Deps(append([]any{gox.FmtCheck, gox.TidyCheck, Vet, fixCheck, Lint}, Config.Checks...)...)
}

// Lint runs golangci-lint.
func Lint() error { return gox.Lint{}.Run() }

// LintFix fixes the issues of the linters where it can.
func LintFix() error { return gox.Lint{Fix: true}.Run() }

// Fmt formats the code with the formatters of golangci-lint.
func Fmt() error { return gox.Fmt() }

// Fix applies the fixes of go fix: Go of before, written with the features
// of now.
func Fix() error { return gox.Fix(Config.Pkgs...) }

// Hints fails on the hints of gopls, those that go fix has not: slow, gopls
// loads the whole module.
func Hints() error { return gox.Hints{Ignore: Config.HintsIgnore}.Run() }

// Align fails on the structs whose fields, in another order, would take
// less memory: a table of them, the most wasteful first.
func Align() error { return gox.FieldAlignment{Pkgs: Config.Pkgs}.Run() }

// AlignFix rearranges the fields of the structs that Align finds; those with
// comments are left, fieldalignment would drop them.
func AlignFix() error { return gox.FieldAlignment{Pkgs: Config.Pkgs, Fix: true}.Run() }

// Vet runs go vet.
func Vet() error { return gox.Vet(Config.Pkgs...) }

// Tidy runs go mod tidy.
func Tidy() error { return gox.Tidy() }

func fixCheck() error { return gox.FixCheck(Config.Pkgs...) }
