package config

import "testing"

func TestLoadFake(t *testing.T) {
	t.Setenv("SUPERVISOR_TOKEN", "")
	t.Setenv("PORT", "9000")
	t.Setenv("DATA_DIR", "/tmp/x")
	c := Load()
	if c.Mode != ModeFake || c.Port != "9000" || c.DataDir != "/tmp/x" {
		t.Fatalf("got %+v", c)
	}
}

func TestLoadSupervisor(t *testing.T) {
	t.Setenv("SUPERVISOR_TOKEN", "abc")
	t.Setenv("PORT", "9000")
	c := Load()
	if c.Mode != ModeSupervisor || c.Port != "8099" || c.DataDir != "/data" || c.SupervisorToken != "abc" {
		t.Fatalf("got %+v", c)
	}
}
