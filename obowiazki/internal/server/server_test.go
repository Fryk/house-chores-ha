package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Fryk/house-chores-ha/obowiazki/internal/config"
	"github.com/Fryk/house-chores-ha/obowiazki/internal/ha"
	"github.com/Fryk/house-chores-ha/obowiazki/internal/store"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func newTestServer(t *testing.T, mode config.Mode, client ha.Client) (*Server, *store.Store) {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	s, err := New(config.Config{Mode: mode}, st, client, quiet)
	if err != nil {
		t.Fatal(err)
	}
	return s, st
}

func do(h http.Handler, method, target string, form url.Values, hdr map[string]string) *httptest.ResponseRecorder {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req := httptest.NewRequest(method, target, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestFakeFlowSendClickClear(t *testing.T) {
	fake := ha.NewFake(quiet)
	s, st := newTestServer(t, config.ModeFake, fake)
	fake.SetHandler(s.HandleAction)
	h := s.Handler()

	rec := do(h, "GET", "/", nil, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), ha.FakeServices[0]) || !strings.Contains(rec.Body.String(), `<base href="/">`) {
		t.Fatalf("index: %d %s", rec.Code, rec.Body.String())
	}

	rec = do(h, "POST", "/test-notify", url.Values{"service": {"notify." + ha.FakeServices[0]}, "click_action": {"/abc_obowiazki"}},
		map[string]string{"Accept-Language": "pl-PL,pl;q=0.9"})
	if rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), "/?msg=") {
		t.Fatalf("test-notify: %d %v", rec.Code, rec.Header())
	}
	sent := fake.Sent()
	if len(sent) != 1 || sent[0].Tag() != "chores-test-1" || sent[0].N.Data["clickAction"] != "/abc_obowiazki" ||
		sent[0].N.Data["channel"] != "Obowiązki" || sent[0].Actions()[0].Title != "Zrobione" || sent[0].N.Data["priority"] != "high" || len(sent[0].Actions()) != 2 {
		t.Fatalf("sent = %+v", sent)
	}
	if p, _, _ := st.Setting(t.Context(), settingPanelPath); p != "/abc_obowiazki" {
		t.Errorf("panel path not saved: %q", p)
	}

	rec = do(h, "GET", "/dev", nil, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "CHORES_DONE_1_p0") {
		t.Fatalf("dev: %d", rec.Code)
	}
	rec = do(h, "POST", "/dev/click", url.Values{"id": {"1"}, "action": {"CHORES_DONE_1_p0"}}, nil)
	if rec.Code != http.StatusSeeOther || strings.Contains(rec.Header().Get("Location"), "err=1") {
		t.Fatalf("click: %d %v", rec.Code, rec.Header())
	}
	if fake.Sent()[0].Visible() {
		t.Error("DONE should clear the notification")
	}
	ev, _ := st.RecentEvents(t.Context(), 10)
	kinds := []string{}
	for _, e := range ev {
		kinds = append(kinds, e.Kind)
	}
	if strings.Join(kinds, ",") != "clear,action,notify" {
		t.Fatalf("events = %v", kinds)
	}
	if ev[1].Action != "CHORES_DONE_1_p0" || ev[1].Tag != "chores-test-1" {
		t.Errorf("action event = %+v", ev[1])
	}
}

func TestTestNotifyRejectsBadService(t *testing.T) {
	fake := ha.NewFake(quiet)
	s, _ := newTestServer(t, config.ModeFake, fake)
	rec := do(s.Handler(), "POST", "/test-notify", url.Values{"service": {"light.kitchen"}}, nil)
	if !strings.Contains(rec.Header().Get("Location"), "err=1") || len(fake.Sent()) != 0 {
		t.Fatalf("expected error redirect, got %v", rec.Header())
	}
}

func TestSupervisorModeIngressRules(t *testing.T) {
	// Fake jako klient, ale tryb supervisor: /dev nie może istnieć.
	s, _ := newTestServer(t, config.ModeSupervisor, ha.NewFake(quiet))
	h := s.Handler()

	rec := do(h, "GET", "/", nil, nil) // httptest: RemoteAddr 192.0.2.1
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-ingress address: %d", rec.Code)
	}

	ingress := func(method, target string, form url.Values) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, target, nil)
		if form != nil {
			req = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		req.RemoteAddr = config.IngressGateway + ":41234"
		req.Header.Set("X-Ingress-Path", "/api/hassio_ingress/TOKEN123")
		req.Header.Set("X-Remote-User-Id", "u-42")
		req.Header.Set("X-Remote-User-Display-Name", "Paweł")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	rec = ingress("GET", "/", nil)
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, `<base href="/api/hassio_ingress/TOKEN123/">`) ||
		!strings.Contains(body, "u-42") || !strings.Contains(body, "Paweł") || strings.Contains(body, `href="dev"`) {
		t.Fatalf("index via ingress: %d\n%s", rec.Code, body)
	}
	if rec := ingress("GET", "/dev", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("/dev in supervisor mode: %d", rec.Code)
	}
	rec = ingress("POST", "/test-notify", url.Values{"service": {ha.FakeServices[0]}})
	if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/api/hassio_ingress/TOKEN123/?") {
		t.Fatalf("redirect must stay under ingress path: %q", loc)
	}
}

func TestBasePathSanitized(t *testing.T) {
	for in, want := range map[string]string{
		"":                          "",
		"/api/hassio_ingress/abc/":  "/api/hassio_ingress/abc",
		`/x"><script>`:              "",
		"//evil.example/x":          "",
		"/api/hassio_ingress/a-b_c": "/api/hassio_ingress/a-b_c",
	} {
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("X-Ingress-Path", in)
		if got := basePath(r); got != want {
			t.Errorf("basePath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIgnoresForeignActions(t *testing.T) {
	s, st := newTestServer(t, config.ModeFake, ha.NewFake(quiet))
	s.HandleAction(ha.ActionEvent{Action: "ALARM", Tag: "x"})
	if ev, _ := st.RecentEvents(t.Context(), 10); len(ev) != 0 {
		t.Fatalf("foreign action logged: %+v", ev)
	}
}

func TestLanguages(t *testing.T) {
	fake := ha.NewFake(quiet)
	s, _ := newTestServer(t, config.ModeFake, fake)
	h := s.Handler()

	rec := do(h, "GET", "/", nil, nil)
	if body := rec.Body.String(); !strings.Contains(body, `<html lang="en">`) || !strings.Contains(body, "Send test notification") {
		t.Fatalf("default should be English:\n%s", body)
	}
	rec = do(h, "GET", "/", nil, map[string]string{"Accept-Language": "pl"})
	if body := rec.Body.String(); !strings.Contains(body, `<html lang="pl">`) || !strings.Contains(body, "Wyślij testowe powiadomienie") {
		t.Fatalf("Accept-Language pl should give Polish")
	}
	rec = do(h, "GET", "/?lang=pl", nil, nil)
	c := rec.Result().Cookies()
	if len(c) != 1 || c[0].Name != "lang" || c[0].Value != "pl" || c[0].Path != "/" {
		t.Fatalf("lang cookie = %+v", c)
	}

	do(h, "POST", "/test-notify", url.Values{"service": {ha.FakeServices[0]}}, nil)
	n := fake.Sent()[0]
	if n.N.Title != "Chores" || n.N.Data["channel"] != "Chores" || n.Actions()[1].Title != "In 1 hour" {
		t.Fatalf("English notification = %+v", n.N)
	}
	if _, ok := n.N.Data["clickAction"]; ok {
		t.Error("empty click_action must not be sent")
	}

	rec = do(h, "POST", "/dev/click", url.Values{"id": {"9"}, "action": {"x"}}, map[string]string{"Accept-Language": "pl"})
	if loc, _ := url.QueryUnescape(rec.Header().Get("Location")); !strings.Contains(loc, "Nie ma takiego powiadomienia") {
		t.Fatalf("translated click error, got %q", loc)
	}
}
