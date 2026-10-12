// The targets of magox, run by magox itself.
package main

import (
	//mage:import
	"github.com/gsontag/magox/targets"
	//mage:import build
	_ "github.com/gsontag/magox/targets/build"
	//mage:import quality
	_ "github.com/gsontag/magox/targets/quality"
	//mage:import test
	_ "github.com/gsontag/magox/targets/test"
)

var Default = targets.All
