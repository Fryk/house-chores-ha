package store

import (
	"context"
	"testing"
)

func TestMigrateSettingsAndEvents(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if v, err := s.SchemaVersion(ctx); err != nil || v != 1 {
		t.Fatalf("SchemaVersion = %d, %v", v, err)
	}
	if err := s.SetSetting(ctx, "panel_path", "/a"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSetting(ctx, "panel_path", "/b"); err != nil {
		t.Fatal(err)
	}
	if v, ok, _ := s.Setting(ctx, "panel_path"); !ok || v != "/b" {
		t.Fatalf("Setting = %q, %v", v, ok)
	}
	if _, ok, _ := s.Setting(ctx, "missing"); ok {
		t.Fatal("missing setting reported as present")
	}
	for want := int64(1); want <= 3; want++ {
		if got, err := s.NextSeq(ctx, "seq"); err != nil || got != want {
			t.Fatalf("NextSeq = %d, %v; want %d", got, err, want)
		}
	}
	s.LogEvent(ctx, Event{Kind: "notify", Service: "mobile_app_a", Tag: "t1"})
	s.LogEvent(ctx, Event{Kind: "notify", Service: "mobile_app_b", Tag: "t1"})
	s.LogEvent(ctx, Event{Kind: "action", Tag: "t1", Action: "CHORES_DONE_1_p0"})
	if svc, ok, _ := s.ServiceForTag(ctx, "t1"); !ok || svc != "mobile_app_b" {
		t.Fatalf("ServiceForTag = %q, %v", svc, ok)
	}
	s.Close()

	// Po ponownym otwarciu dane zostają, a migracje nie są stosowane drugi raz.
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ev, err := s.RecentEvents(ctx, 10)
	if err != nil || len(ev) != 3 || ev[0].Kind != "action" || ev[0].At.IsZero() {
		t.Fatalf("RecentEvents = %+v, %v", ev, err)
	}
	if got, _ := s.NextSeq(ctx, "seq"); got != 4 {
		t.Fatalf("NextSeq after reopen = %d", got)
	}
}
