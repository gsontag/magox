package test

import "testing"

func TestDefaults(t *testing.T) {
	if !Config.Race || !Config.ExcludeGenerated || Config.Profile != "coverage.out" {
		t.Errorf("Config = %+v", Config)
	}
}
