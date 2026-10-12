// Package targets gives the targets that every project in Go needs, to import
// in a magefile:
//
//	import (
//		//mage:import
//		"github.com/gsontag/magox/targets"
//	)
//
//	func init() {
//		targets.Config.Platforms = []string{"windows/amd64", "js/wasm"}
//	}
//
// With an alias, //mage:import go, they become go:test, go:lint...: what lets
// the magefile write a target of the same name.
//
// Mage makes a target of each exported function here: the package has no
// other.
package targets

import (
	"time"

	"github.com/magefile/mage/mg"

	"github.com/gsontag/magox/gox"
)

// Settings tells the targets what the project is. The magefile changes
// Config in its init: mage runs the targets after.
type Settings struct {
	Pkgs []string // the packages built, tested and vetted; ./... if empty
	// Builds are the binaries that Build writes; empty, Build compiles Pkgs
	Builds []gox.Build
	// Platforms are the os/arch that Cross builds CrossPkgs for
	Platforms []string
	CrossPkgs []string // Pkgs if empty

	Race             bool
	Timeout          time.Duration
	Profile          string  // the coverage profile
	ExcludeGenerated bool    // the generated files out of the coverage
	MinCoverage      float64 // Coverage fails below, in percent; 0 for none

	// HintsIgnore drops the hints of gopls that contain one of these:
	// "[darwin]" for those of another platform
	HintsIgnore []string
}

// Config is what the targets run with.
var Config = Settings{
	Race:             true,
	Profile:          "coverage.out",
	ExcludeGenerated: true,
}

// Build compiles the packages, or writes the binaries of Config.Builds.
func Build() error {
	if len(Config.Builds) == 0 {
		return gox.Build{Pkgs: pkgs()}.Run()
	}
	for _, b := range Config.Builds {
		if err := b.Run(); err != nil {
			return err
		}
	}
	return nil
}

// Cross builds the packages for the platforms of Config.Platforms: a
// dependency on one OS slipped into them breaks there.
func Cross() error {
	if len(Config.Platforms) == 0 {
		return nil
	}
	p := Config.CrossPkgs
	if len(p) == 0 {
		p = pkgs()
	}
	return gox.Build{Pkgs: p, Platforms: Config.Platforms}.Run()
}

// Test runs the tests, with a coverage profile.
func Test() error {
	return gox.Test{
		Pkgs:             pkgs(),
		Race:             Config.Race,
		Timeout:          Config.Timeout,
		Cover:            Config.Profile,
		ExcludeGenerated: Config.ExcludeGenerated,
	}.Run()
}

// Coverage shows the coverage of each package, after the tests.
func Coverage() error {
	mg.Deps(Test)
	return gox.Coverage{Profile: Config.Profile, Packages: true, Min: Config.MinCoverage}.Run()
}

// CoverageHTML opens the coverage report in the browser, after the tests.
func CoverageHTML() error {
	mg.Deps(Test)
	return gox.CoverHTML(Config.Profile)
}

// Lint runs golangci-lint.
func Lint() error { return gox.Lint{}.Run() }

// LintFix fixes the issues of the linters where it can.
func LintFix() error { return gox.Lint{Fix: true}.Run() }

// Fmt formats the code with the formatters of golangci-lint.
func Fmt() error { return gox.Fmt() }

// Fix applies the fixes of go fix: Go of before, written with the features
// of now.
func Fix() error { return gox.Fix(pkgs()...) }

// Hints fails on the hints of gopls, those that go fix has not: slow, gopls
// loads the whole module.
func Hints() error { return gox.Hints{Ignore: Config.HintsIgnore}.Run() }

// Vet runs go vet.
func Vet() error { return gox.Vet(pkgs()...) }

// Tidy runs go mod tidy.
func Tidy() error { return gox.Tidy() }

// Check runs the checks of quality, without changing a file: format,
// go.mod, vet, go fix, lint.
func Check() {
	mg.Deps(gox.FmtCheck, gox.TidyCheck, Vet, fixCheck, Lint)
}

func fixCheck() error { return gox.FixCheck(pkgs()...) }

// All runs the checks, the tests with their coverage, and the cross builds.
func All() {
	mg.Deps(Check, Coverage, Cross)
}

func pkgs() []string {
	if len(Config.Pkgs) == 0 {
		return []string{"./..."}
	}
	return Config.Pkgs
}
