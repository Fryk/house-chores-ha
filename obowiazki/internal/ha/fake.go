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

// Fake udaje HA i telefony: zapamiętuje powiadomienia (z zastępowaniem i kasowaniem po tagu,
// jak aplikacja Companion na Androidzie) i pozwala „kliknąć” ich akcje ze strony /dev.
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
	Replaced bool // zastąpione nowszym z tym samym tagiem
	Cleared  bool // skasowane przez clear_notification
}

// Visible mówi, czy powiadomienie nadal wisiałoby na telefonie.
func (f FakeNotification) Visible() bool { return !f.Replaced && !f.Cleared }

func (f FakeNotification) Tag() string { s, _ := f.N.Data["tag"].(string); return s }

type FakeAction struct{ Action, Title string }

// Actions zwraca przyciski powiadomienia niezależnie od tego, czy przyszły jako []map, czy []FakeAction.
func (f FakeNotification) Actions() []FakeAction {
	raw, err := json.Marshal(f.N.Data["actions"])
	if err != nil {
		return nil
	}
	var out []FakeAction
	json.Unmarshal(raw, &out)
	return out
}

var FakeServices = []string{"mobile_app_telefon_testowy_1", "mobile_app_telefon_testowy_2"}

func NewFake(log *slog.Logger) *Fake { return &Fake{now: time.Now, log: log} }

func (f *Fake) Notify(_ context.Context, service string, n Notification) error {
	if !strings.HasPrefix(service, "mobile_app_") {
		return fmt.Errorf("nieznana usługa notify.%s", service)
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
		f.log.Info("fake: kasowanie powiadomienia", "service", service, "tag", tag)
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
	f.log.Info("fake: powiadomienie", "service", service, "payload", string(payload))
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

// SetHandler ustawia odbiorcę kliknięć bez blokowania (Run robi to samo i czeka na ctx).
func (f *Fake) SetHandler(onAction func(ActionEvent)) {
	f.mu.Lock()
	f.onAction = onAction
	f.mu.Unlock()
}

// Sent zwraca kopię wysłanych powiadomień, od najnowszego.
func (f *Fake) Sent() []FakeNotification {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]FakeNotification, len(f.sent))
	for i, n := range f.sent {
		out[len(f.sent)-1-i] = n
	}
	return out
}

// Click symuluje tapnięcie przycisku akcji w powiadomieniu o danym ID.
func (f *Fake) Click(id int, action string) error {
	f.mu.Lock()
	if id < 1 || id > len(f.sent) {
		f.mu.Unlock()
		return errors.New("nie ma takiego powiadomienia")
	}
	n := f.sent[id-1]
	handler := f.onAction
	f.mu.Unlock()
	if !n.Visible() {
		return errors.New("tego powiadomienia nie ma już na telefonie")
	}
	found := false
	for _, a := range n.Actions() {
		found = found || a.Action == action
	}
	if !found {
		return fmt.Errorf("powiadomienie nie ma akcji %q", action)
	}
	if handler == nil {
		return errors.New("odbiór zdarzeń jeszcze nie działa")
	}
	// Taki sam kształt danych jak w zdarzeniu z Androida: action + wszystkie pola data.
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
