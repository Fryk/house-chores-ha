package actions

import "testing"

func TestRoundTrip(t *testing.T) {
	for _, a := range []Action{{Done, 123, 2}, {Snooze, 0, 0}, {Me, 9, 11}} {
		got, ok := Parse(a.String())
		if !ok || got != a {
			t.Errorf("Parse(%q) = %+v, %v", a.String(), got, ok)
		}
	}
	if s := (Action{Done, 123, 2}).String(); s != "CHORES_DONE_123_p2" {
		t.Errorf("String() = %q", s)
	}
}

func TestParseRejects(t *testing.T) {
	for _, s := range []string{
		"", "ALARM", "CHORES_", "CHORES_DONE_1", "CHORES_FOO_1_p2", "CHORES_DONE_x_p2",
		"CHORES_DONE_1_2", "CHORES_DONE_1_px", "CHORES_DONE_-1_p2", "CHORES_DONE_1_p2_extra",
	} {
		if a, ok := Parse(s); ok {
			t.Errorf("Parse(%q) = %+v, want reject", s, a)
		}
	}
}
