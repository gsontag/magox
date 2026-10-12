// Package test gives the targets that run the tests, to import in a
// magefile under a namespace:
//
//	import (
//		//mage:import test
//		"github.com/gsontag/magox/targets/test"
//	)
//
// They are then test:run, test:coverage, test:coverageHTML; the magefile
// adds its own to the namespace with a type Test mg.Namespace.
//
// Mage makes a target of each exported function here: the package has no
// other.
package test

import (
	"time"

	"github.com/magefile/mage/mg"

	"github.com/gsontag/magox/gox"
)

// Settings tells the targets what the project is. The magefile changes
// Config in its init: mage runs the targets after.
type Settings struct {
	Pkgs             []string // the packages tested; ./... if empty
	Race             bool
	Timeout          time.Duration
	Profile          string  // the coverage profile
	ExcludeGenerated bool    // the generated files out of the coverage
	MinCoverage      float64 // Coverage fails below, in percent; 0 for none
}

// Config is what the targets run with.
var Config = Settings{
	Race:             true,
	Profile:          "coverage.out",
	ExcludeGenerated: true,
}

// Run runs the tests, with a coverage profile.
func Run() error {
	return gox.Test{
		Pkgs:             Config.Pkgs,
		Race:             Config.Race,
		Timeout:          Config.Timeout,
		Cover:            Config.Profile,
		ExcludeGenerated: Config.ExcludeGenerated,
	}.Run()
}

// Coverage shows the coverage of each package, after the tests.
func Coverage() error {
	mg.Deps(Run)
	return gox.Coverage{Profile: Config.Profile, Packages: true, Min: Config.MinCoverage}.Run()
}

// CoverageHTML opens the coverage report in the browser, after the tests.
func CoverageHTML() error {
	mg.Deps(Run)
	return gox.CoverHTML(Config.Profile)
}
