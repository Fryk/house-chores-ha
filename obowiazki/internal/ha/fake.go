package ha

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Fake imitates Home Assistant and the phones: it keeps notifications (replacing and clearing
// them by tag, like the Android Companion app does) and lets the /dev page tap their actions.
type Fake struct {
	mu       sync.Mutex
	sent     []FakeNotification
	onAction func(ActionEvent)
	now      func() time.Time
	log      *slog.Logger
}

type FakeNotification struct {
	ID       int
	Service  string
	At       time.Time
	N        Notification
	Replaced bool // replaced by a newer one with the same tag
	Cleared  bool // removed by clear_notification
}

// Visible reports whether the notification would still be shown on the phone.
func (f FakeNotification) Visible() bool { return !f.Replaced && !f.Cleared }

func (f FakeNotification) Tag() string { s, _ := f.N.Data["tag"].(string); return s }

type FakeAction struct{ Action, Title string }

// Actions returns the notification buttons whatever Go type they were sent as.
func (f FakeNotification) Actions() []FakeAction {
	raw, err := json.Marshal(f.N.Data["actions"])
	if err != nil {
		return nil
	}
	var out []FakeAction
	json.Unmarshal(raw, &out)
	return out
}

var FakeServices = []string{"mobile_app_test_phone_1", "mobile_app_test_phone_2"}

var (
	ErrNoSuchNotification = errors.New("no such notification")
	ErrNotificationGone   = errors.New("notification is no longer shown")
	ErrNoSuchAction       = errors.New("notification has no such action")
	ErrNotRunning         = errors.New("event handling is not running yet")
)

func NewFake(log *slog.Logger) *Fake { return &Fake{now: time.Now, log: log} }

func (f *Fake) Notify(_ context.Context, service string, n Notification) error {
	if !strings.HasPrefix(service, "mobile_app_") {
		return fmt.Errorf("unknown service notify.%s", service)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	tag, _ := n.Data["tag"].(string)
	if n.Message == ClearMessage {
		for i := range f.sent {
			if f.sent[i].Service == service && tag != "" && f.sent[i].Tag() == tag {
				f.sent[i].Cleared = true
			}
		}
		f.log.Info("fake: clear notification", "service", service, "tag", tag)
		return nil
	}
	for i := range f.sent {
		if f.sent[i].Service == service && tag != "" && f.sent[i].Tag() == tag {
			f.sent[i].Replaced = true
		}
	}
	fn := FakeNotification{ID: len(f.sent) + 1, Service: service, At: f.now(), N: n}
	f.sent = append(f.sent, fn)
	payload, _ := json.Marshal(n)
	f.log.Info("fake: notification", "service", service, "payload", string(payload))
	return nil
}

func (f *Fake) NotifyServices(context.Context) ([]string, error) {
	return append([]string(nil), FakeServices...), nil
}

func (f *Fake) SelfInfo(context.Context) (SelfInfo, error) {
	return SelfInfo{Slug: "local_obowiazki"}, nil
}

func (f *Fake) Run(ctx context.Context, onAction func(ActionEvent)) error {
	f.SetHandler(onAction)
	<-ctx.Done()
	return nil
}

// SetHandler sets the tap receiver without blocking (Run does the same and waits for ctx).
func (f *Fake) SetHandler(onAction func(ActionEvent)) {
	f.mu.Lock()
	f.onAction = onAction
	f.mu.Unlock()
}

// Sent returns a copy of the sent notifications, newest first.
func (f *Fake) Sent() []FakeNotification {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]FakeNotification, len(f.sent))
	for i, n := range f.sent {
		out[len(f.sent)-1-i] = n
	}
	return out
}

// Click simulates tapping an action button of the notification with the given ID.
func (f *Fake) Click(id int, action string) error {
	f.mu.Lock()
	if id < 1 || id > len(f.sent) {
		f.mu.Unlock()
		return ErrNoSuchNotification
	}
	n := f.sent[id-1]
	handler := f.onAction
	f.mu.Unlock()
	if !n.Visible() {
		return ErrNotificationGone
	}
	found := false
	for _, a := range n.Actions() {
		found = found || a.Action == action
	}
	if !found {
		return ErrNoSuchAction
	}
	if handler == nil {
		return ErrNotRunning
	}
	// Same data shape as the Android event: action plus every data field.
	data := map[string]any{"action": action}
	for k, v := range n.N.Data {
		if k != "actions" {
			data[k] = v
		}
	}
	raw, _ := json.Marshal(map[string]any{"event_type": "mobile_app_notification_action", "data": data})
	handler(ActionEvent{Action: action, Tag: n.Tag(), UserID: "fake-user", At: f.now(), Raw: raw})
	return nil
}
