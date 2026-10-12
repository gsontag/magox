// Package targets gives the targets that every project in Go needs, in a
// package by domain, to import in a magefile under its namespace:
//
//	import (
//		//mage:import
//		"github.com/gsontag/magox/targets"
//		//mage:import build
//		"github.com/gsontag/magox/targets/build"
//		//mage:import quality
//		"github.com/gsontag/magox/targets/quality"
//		//mage:import test
//		"github.com/gsontag/magox/targets/test"
//	)
//
// They are then all, build:run, build:cross, quality:check, quality:lint,
// test:run, test:coverage... Each namespace takes the targets of the
// project too: a method of type Quality mg.Namespace in the magefile
// becomes quality:something. The settings are in the Config of each
// package.
package targets

import (
	"github.com/magefile/mage/mg"

	"github.com/gsontag/magox/targets/build"
	"github.com/gsontag/magox/targets/quality"
	"github.com/gsontag/magox/targets/test"
)

// All runs the checks, the tests with their coverage, and the cross builds.
func All() {
	mg.Deps(quality.Check, test.Coverage, build.Cross)
}
