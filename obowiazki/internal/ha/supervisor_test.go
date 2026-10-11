package ha

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// fakeHA imitates the Supervisor proxy as far as the app uses it.
type fakeHA struct {
	t          *testing.T
	lastPath   string
	lastAuth   string
	lastBody   map[string]any
	services   string
	conns      atomic.Int32
	subscribed chan map[string]any
}

func (f *fakeHA) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.lastPath, f.lastAuth = r.URL.Path, r.Header.Get("Authorization")
	switch r.URL.Path {
	case "/core/api/services/notify/mobile_app_pixel":
		json.NewDecoder(r.Body).Decode(&f.lastBody)
		io.WriteString(w, "[]")
	case "/core/api/services":
		io.WriteString(w, f.services)
	case "/addons/self/info":
		io.WriteString(w, `{"result":"ok","data":{"slug":"abc123_obowiazki","ingress_url":"/api/hassio_ingress/xyz/"}}`)
	case "/core/websocket":
		f.ws(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeHA) ws(w http.ResponseWriter, r *http.Request) {
	n := f.conns.Add(1)
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer c.CloseNow()
	ctx := r.Context()
	wsjson.Write(ctx, c, map[string]any{"type": "auth_required", "ha_version": "2026.10.0"})
	var auth map[string]any
	if wsjson.Read(ctx, c, &auth) != nil {
		return
	}
	if auth["type"] != "auth" || auth["access_token"] != "tok" {
		wsjson.Write(ctx, c, map[string]any{"type": "auth_invalid", "message": "Invalid access token"})
		return
	}
	wsjson.Write(ctx, c, map[string]any{"type": "auth_ok", "ha_version": "2026.10.0"})
	var sub map[string]any
	if wsjson.Read(ctx, c, &sub) != nil {
		return
	}
	f.subscribed <- sub
	id := sub["id"]
	wsjson.Write(ctx, c, map[string]any{"id": id, "type": "result", "success": true, "result": nil})
	wsjson.Write(ctx, c, map[string]any{"id": id, "type": "event", "event": map[string]any{
		"event_type": "mobile_app_notification_action",
		"data":       map[string]any{"action": "CHORES_DONE_7_p0", "tag": "chores-test-7", "channel": "Obowiązki"},
		"origin":     "REMOTE",
		"time_fired": "2026-10-11T10:00:00+00:00",
		"context":    map[string]any{"id": "x", "user_id": "u1"},
	}})
	if n == 1 {
		return // drop the first connection to exercise reconnecting
	}
	<-ctx.Done()
}

func newFakeHA(t *testing.T) (*fakeHA, *Supervisor) {
	f := &fakeHA{t: t, subscribed: make(chan map[string]any, 4),
		services: `[{"domain":"light","services":{"turn_on":{}}},{"domain":"notify","services":{"mobile_app_pixel":{},"notify":{},"mobile_app_a52":{}}}]`}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, NewSupervisor(srv.URL, "tok", quiet)
}

func TestSupervisorNotify(t *testing.T) {
	f, s := newFakeHA(t)
	n := Notification{Title: "Obowiązki", Message: "Test", Data: map[string]any{"tag": "t1"}}
	if err := s.Notify(context.Background(), "mobile_app_pixel", n); err != nil {
		t.Fatal(err)
	}
	if f.lastAuth != "Bearer tok" {
		t.Errorf("Authorization = %q", f.lastAuth)
	}
	// The body is the plain service_data, no wrapper.
	if f.lastBody["message"] != "Test" || f.lastBody["data"].(map[string]any)["tag"] != "t1" {
		t.Errorf("body = %v", f.lastBody)
	}
	if err := s.Notify(context.Background(), "mobile_app_missing", n); err == nil {
		t.Error("expected error for 404")
	}
}

func TestSupervisorNotifyServices(t *testing.T) {
	f, s := newFakeHA(t)
	for _, body := range []string{
		f.services,
		`[{"domain":"notify","services":["mobile_app_pixel","persistent_notification","mobile_app_a52"]}]`,
	} {
		f.services = body
		got, err := s.NotifyServices(context.Background())
		if err != nil || len(got) != 2 || got[0] != "mobile_app_a52" || got[1] != "mobile_app_pixel" {
			t.Errorf("NotifyServices(%s) = %v, %v", body, got, err)
		}
	}
}

func TestSupervisorSelfInfo(t *testing.T) {
	_, s := newFakeHA(t)
	info, err := s.SelfInfo(context.Background())
	if err != nil || info.Slug != "abc123_obowiazki" || info.IngressURL != "/api/hassio_ingress/xyz/" {
		t.Fatalf("SelfInfo = %+v, %v", info, err)
	}
}

func TestSupervisorRunReceivesActionsAndReconnects(t *testing.T) {
	f, s := newFakeHA(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	got := make(chan ActionEvent, 4)
	done := make(chan struct{})
	go func() { s.Run(ctx, func(e ActionEvent) { got <- e }); close(done) }()

	for i := 0; i < 2; i++ { // the second event arrives after reconnecting
		select {
		case sub := <-f.subscribed:
			if sub["type"] != "subscribe_events" || sub["event_type"] != "mobile_app_notification_action" {
				t.Fatalf("subscribe = %v", sub)
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for subscribe")
		}
		select {
		case e := <-got:
			if e.Action != "CHORES_DONE_7_p0" || e.Tag != "chores-test-7" || e.UserID != "u1" || e.At.IsZero() {
				t.Fatalf("event = %+v", e)
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for event")
		}
	}
	cancel()
	<-done
}

func TestSupervisorAuthInvalid(t *testing.T) {
	f, s := newFakeHA(t)
	s.token = "zly"
	if err := s.session(context.Background(), func(ActionEvent) {}); err == nil {
		t.Fatal("expected auth error")
	}
	_ = f
}
