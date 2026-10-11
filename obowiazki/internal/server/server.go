// Package server to interfejs HTTP appki: strona główna (ingress), test powiadomień i strona /dev w trybie fake.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Fryk/house-chores-ha/obowiazki/internal/actions"
	"github.com/Fryk/house-chores-ha/obowiazki/internal/config"
	"github.com/Fryk/house-chores-ha/obowiazki/internal/ha"
	"github.com/Fryk/house-chores-ha/obowiazki/internal/store"
	"github.com/Fryk/house-chores-ha/obowiazki/web"
)

const (
	settingPanelPath   = "panel_path"
	settingLastService = "test_service"
	channelMain        = "Obowiązki"
)

type Server struct {
	cfg    config.Config
	store  *store.Store
	client ha.Client
	fake   *ha.Fake // nil w trybie supervisor
	log    *slog.Logger
	now    func() time.Time
	pages  map[string]*template.Template

	selfOnce sync.Once
	self     ha.SelfInfo
}

func New(cfg config.Config, st *store.Store, client ha.Client, log *slog.Logger) (*Server, error) {
	s := &Server{cfg: cfg, store: st, client: client, log: log, now: time.Now, pages: map[string]*template.Template{}}
	if f, ok := client.(*ha.Fake); ok && cfg.Mode == config.ModeFake {
		s.fake = f
	}
	for _, p := range []string{"index", "dev"} {
		t, err := template.ParseFS(web.FS, "templates/layout.html", "templates/"+p+".html")
		if err != nil {
			return nil, err
		}
		s.pages[p] = t
	}
	return s, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	static, _ := fs.Sub(web.FS, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("POST /test-notify", s.handleTestNotify)
	if s.fake != nil {
		mux.HandleFunc("GET /dev", s.handleDev)
		mux.HandleFunc("POST /dev/click", s.handleDevClick)
	}
	if s.cfg.Mode == config.ModeSupervisor {
		return onlyFrom(config.IngressGateway, mux)
	}
	return mux
}

// onlyFrom przepuszcza tylko połączenia z bramy ingress (wymóg z dokumentacji HA).
func onlyFrom(ip string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil || host != ip {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

var safePath = regexp.MustCompile(`^(/[A-Za-z0-9._~-]+)+$`)

// basePath zwraca ścieżkę bazową z nagłówka X-Ingress-Path (bez końcowego "/"), a w trybie fake "".
func basePath(r *http.Request) string {
	p := strings.TrimRight(r.Header.Get("X-Ingress-Path"), "/")
	if !safePath.MatchString(p) {
		return ""
	}
	return p
}

type remoteUser struct{ ID, Name, DisplayName string }

func userFrom(r *http.Request) remoteUser {
	return remoteUser{
		ID:          r.Header.Get("X-Remote-User-Id"),
		Name:        r.Header.Get("X-Remote-User-Name"),
		DisplayName: r.Header.Get("X-Remote-User-Display-Name"),
	}
}

type pageData struct {
	Base, Mode, Msg string
	MsgErr, Fake    bool
	User            remoteUser
	Now             time.Time

	Services           []string
	ServicesErr        string
	Service, PanelPath string
	Slug, IngressURL   string
	SchemaVersion      int
	Events             []store.Event
	Sent               []ha.FakeNotification
}

func (s *Server) basePage(r *http.Request) pageData {
	q := r.URL.Query()
	return pageData{
		Base: basePath(r), Mode: string(s.cfg.Mode), Fake: s.fake != nil,
		Msg: q.Get("msg"), MsgErr: q.Get("err") == "1",
		User: userFrom(r), Now: s.now(),
	}
}

func (s *Server) render(w http.ResponseWriter, page string, d pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.pages[page].ExecuteTemplate(w, "layout", d); err != nil {
		s.log.Error("render", "page", page, "err", err)
	}
}

func (s *Server) redirect(w http.ResponseWriter, r *http.Request, to, msg string, isErr bool) {
	v := url.Values{}
	if msg != "" {
		v.Set("msg", msg)
	}
	if isErr {
		v.Set("err", "1")
	}
	loc := basePath(r) + "/" + to
	if len(v) > 0 {
		loc += "?" + v.Encode()
	}
	http.Redirect(w, r, loc, http.StatusSeeOther)
}

func (s *Server) selfInfo(ctx context.Context) ha.SelfInfo {
	s.selfOnce.Do(func() {
		info, err := s.client.SelfInfo(ctx)
		if err != nil {
			s.log.Warn("nie udało się pobrać /addons/self/info", "err", err)
		}
		s.self = info
	})
	return s.self
}

func (s *Server) panelPath(ctx context.Context) string {
	if p, ok, _ := s.store.Setting(ctx, settingPanelPath); ok {
		return p
	}
	if slug := s.selfInfo(ctx).Slug; slug != "" {
		return "/" + slug
	}
	return ""
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	d := s.basePage(r)
	var err error
	if d.Services, err = s.client.NotifyServices(ctx); err != nil {
		d.ServicesErr = err.Error()
	}
	d.Service, _, _ = s.store.Setting(ctx, settingLastService)
	if d.Service == "" && len(d.Services) > 0 {
		d.Service = d.Services[0]
	}
	d.PanelPath = s.panelPath(ctx)
	info := s.selfInfo(ctx)
	d.Slug, d.IngressURL = info.Slug, info.IngressURL
	d.SchemaVersion, _ = s.store.SchemaVersion(ctx)
	if d.Events, err = s.store.RecentEvents(ctx, 20); err != nil {
		s.log.Error("RecentEvents", "err", err)
	}
	s.render(w, "index", d)
}

var serviceName = regexp.MustCompile(`^mobile_app_[a-z0-9_]+$`)

// TestNotification buduje testowe powiadomienie z dwiema akcjami, jak w pkt 5 specyfikacji.
func TestNotification(seq int64, clickAction string) ha.Notification {
	data := map[string]any{
		"tag": fmt.Sprintf("chores-test-%d", seq),
		"actions": []map[string]string{
			{"action": actions.Action{Kind: actions.Done, OccurrenceID: seq}.String(), "title": "Zrobione"},
			{"action": actions.Action{Kind: actions.Snooze, OccurrenceID: seq}.String(), "title": "Za godzinę"},
		},
		"ttl":        0,
		"priority":   "high",
		"channel":    channelMain,
		"importance": "high", // ustawiane tylko przy pierwszym użyciu kanału
	}
	if clickAction != "" {
		data["clickAction"] = clickAction // Android
		data["url"] = clickAction         // iOS
	}
	return ha.Notification{Title: "Obowiązki", Message: fmt.Sprintf("Testowe powiadomienie #%d", seq), Data: data}
}

func (s *Server) handleTestNotify(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	service := strings.TrimPrefix(strings.TrimSpace(r.FormValue("service")), "notify.")
	click := strings.TrimSpace(r.FormValue("click_action"))
	if !serviceName.MatchString(service) {
		s.redirect(w, r, "", "Podaj usługę w postaci mobile_app_…", true)
		return
	}
	s.store.SetSetting(ctx, settingLastService, service)
	s.store.SetSetting(ctx, settingPanelPath, click)
	seq, err := s.store.NextSeq(ctx, "test_seq")
	if err != nil {
		s.redirect(w, r, "", "Błąd bazy: "+err.Error(), true)
		return
	}
	n := TestNotification(seq, click)
	tag := n.Data["tag"].(string)
	payload, _ := json.Marshal(n)
	if err := s.client.Notify(ctx, service, n); err != nil {
		s.log.Error("wysyłka testowego powiadomienia", "service", service, "err", err)
		s.store.LogEvent(ctx, store.Event{Kind: "error", Service: service, Tag: tag, Detail: err.Error()})
		s.redirect(w, r, "", "Nie udało się wysłać: "+err.Error(), true)
		return
	}
	s.log.Info("wysłano testowe powiadomienie", "service", service, "tag", tag)
	s.store.LogEvent(ctx, store.Event{Kind: "notify", Service: service, Tag: tag, Detail: string(payload)})
	s.redirect(w, r, "", fmt.Sprintf("Wysłano testowe powiadomienie #%d na notify.%s", seq, service), false)
}

// HandleAction obsługuje kliknięcie przycisku w powiadomieniu (z HA albo ze strony /dev).
func (s *Server) HandleAction(ev ha.ActionEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	a, ok := actions.Parse(ev.Action)
	if !ok {
		s.log.Debug("pomijam obcą akcję", "action", ev.Action)
		return
	}
	s.log.Info("kliknięto akcję", "kind", a.Kind, "occurrence", a.OccurrenceID, "person", a.PersonID,
		"tag", ev.Tag, "ha_user_id", ev.UserID)
	detail := fmt.Sprintf("%s, wystąpienie %d, osoba %d, użytkownik HA %q", a.Kind, a.OccurrenceID, a.PersonID, ev.UserID)
	if a.Kind == actions.Snooze {
		detail += " — odkładanie przypomnień dojdzie w M2"
	}
	s.store.LogEvent(ctx, store.Event{Kind: "action", Tag: ev.Tag, Action: ev.Action, Detail: detail})

	if a.Kind != actions.Done || ev.Tag == "" {
		return
	}
	// Po „Zrobione” kasujemy powiadomienie z tym tagiem (sprawdzenie clear_notification).
	service, ok, err := s.store.ServiceForTag(ctx, ev.Tag)
	if err != nil || !ok {
		return
	}
	if err := s.client.Notify(ctx, service, ha.Clear(ev.Tag)); err != nil {
		s.log.Error("kasowanie powiadomienia", "service", service, "tag", ev.Tag, "err", err)
		s.store.LogEvent(ctx, store.Event{Kind: "error", Service: service, Tag: ev.Tag, Detail: err.Error()})
		return
	}
	s.store.LogEvent(ctx, store.Event{Kind: "clear", Service: service, Tag: ev.Tag})
}

func (s *Server) handleDev(w http.ResponseWriter, r *http.Request) {
	d := s.basePage(r)
	d.Sent = s.fake.Sent()
	s.render(w, "dev", d)
}

func (s *Server) handleDevClick(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.FormValue("id"))
	action := r.FormValue("action")
	if err := s.fake.Click(id, action); err != nil {
		s.redirect(w, r, "dev", err.Error(), true)
		return
	}
	s.redirect(w, r, "dev", "Kliknięto "+action+" — szczegóły w logu i na stronie głównej", false)
}
