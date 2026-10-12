package ha

import (
	"context"
	"testing"
)

func testNotification(tag string) Notification {
	return Notification{Message: "m", Data: map[string]any{
		"tag":     tag,
		"actions": []map[string]string{{"action": "CHORES_DONE_1_p0", "title": "Zrobione"}},
	}}
}

func TestFakeReplaceClearAndClick(t *testing.T) {
	ctx := context.Background()
	f := NewFake(quiet)
	var got []ActionEvent
	f.SetHandler(func(e ActionEvent) { got = append(got, e) })

	svc := FakeServices[0]
	f.Notify(ctx, svc, testNotification("t1"))
	f.Notify(ctx, svc, testNotification("t1")) // replaces the first one
	f.Notify(ctx, FakeServices[1], testNotification("t1"))
	if err := f.Notify(ctx, "persistent_notification", testNotification("x")); err == nil {
		t.Error("expected error for non mobile_app service")
	}
	sent := f.Sent()
	if len(sent) != 3 || sent[2].Visible() || !sent[1].Visible() {
		t.Fatalf("after replace: %+v", sent)
	}
	if err := f.Click(1, "CHORES_DONE_1_p0"); err == nil {
		t.Error("click on replaced notification should fail")
	}
	if err := f.Click(2, "CHORES_SNOOZE_1_p0"); err == nil {
		t.Error("click on unknown action should fail")
	}
	if err := f.Click(2, "CHORES_DONE_1_p0"); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Tag != "t1" || got[0].Action != "CHORES_DONE_1_p0" {
		t.Fatalf("events = %+v", got)
	}
	f.Notify(ctx, svc, Clear("t1"))
	sent = f.Sent()
	if sent[1].Visible() || !sent[0].Visible() {
		t.Fatalf("clear should hit only the first service: %+v", sent)
	}
}
