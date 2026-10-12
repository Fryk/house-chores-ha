// Package server is the app's HTTP interface: the home page (served through ingress),
// the test notification and, in fake mode, the /dev page.
package server

import (
	"context"
	"encoding/json"
	"errors"
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

	"github.com/Fryk/house-chores-ha/house_chores/internal/actions"
	"github.com/Fryk/house-chores-ha/house_chores/internal/config"
	"github.com/Fryk/house-chores-ha/house_chores/internal/ha"
	"github.com/Fryk/house-chores-ha/house_chores/internal/i18n"
	"github.com/Fryk/house-chores-ha/house_chores/internal/store"
	"github.com/Fryk/house-chores-ha/house_chores/web"
)

const (
	settingPanelPath   = "panel_path"
	settingLastService = "test_service"
)

type Server struct {
	cfg    config.Config
	store  *store.Store
	client ha.Client
	fake   *ha.Fake // nil in supervisor mode
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

// onlyFrom lets through only connections from the ingress gateway, as the Home Assistant docs require.
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

// basePath returns the base path from the X-Ingress-Path header (no trailing "/"), or "" in fake mode.
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

// lang picks the language for the request and remembers an explicit ?lang= choice in a cookie.
func lang(w http.ResponseWriter, r *http.Request) i18n.Lang {
	l := i18n.FromRequest(r)
	if _, ok := i18n.Parse(r.URL.Query().Get("lang")); ok {
		http.SetCookie(w, &http.Cookie{Name: i18n.CookieName, Value: string(l), Path: basePath(r) + "/",
			MaxAge: 365 * 24 * 3600, SameSite: http.SameSiteLaxMode, HttpOnly: true})
	}
	return l
}

type pageData struct {
	L               i18n.Lang
	OtherLang       i18n.Lang
	Base, Mode, Msg string
	MsgErr, Fake    bool
	User            remoteUser
	Now             time.Time

	Services           []string
	ServicesErr        string
	Service, PanelPath string
	DefaultPanelPath   string
	Slug, IngressURL   string
	SchemaVersion      int
	Events             []store.Event
	Sent               []ha.FakeNotification
}

func (s *Server) basePage(w http.ResponseWriter, r *http.Request) pageData {
	q := r.URL.Query()
	l := lang(w, r)
	other := i18n.EN
	if l == i18n.EN {
		other = i18n.PL
	}
	return pageData{
		L: l, OtherLang: other,
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
			s.log.Warn("could not read /addons/self/info", "err", err)
		}
		s.self = info
	})
	return s.self
}

// defaultPanelPath is a guess ("/<slug>") until the real panel path is confirmed in M0.
func (s *Server) defaultPanelPath(ctx context.Context) string {
	if slug := s.selfInfo(ctx).Slug; slug != "" {
		return "/" + slug
	}
	return ""
}

func (s *Server) panelPath(ctx context.Context) string {
	if p, ok, _ := s.store.Setting(ctx, settingPanelPath); ok {
		return p
	}
	return s.defaultPanelPath(ctx)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	d := s.basePage(w, r)
	var err error
	if d.Services, err = s.client.NotifyServices(ctx); err != nil {
		d.ServicesErr = err.Error()
	}
	d.Service, _, _ = s.store.Setting(ctx, settingLastService)
	if d.Service == "" && len(d.Services) > 0 {
		d.Service = d.Services[0]
	}
	d.PanelPath = s.panelPath(ctx)
	d.DefaultPanelPath = s.defaultPanelPath(ctx)
	info := s.selfInfo(ctx)
	d.Slug, d.IngressURL = info.Slug, info.IngressURL
	d.SchemaVersion, _ = s.store.SchemaVersion(ctx)
	if d.Events, err = s.store.RecentEvents(ctx, 20); err != nil {
		s.log.Error("RecentEvents", "err", err)
	}
	s.render(w, "index", d)
}

var serviceName = regexp.MustCompile(`^mobile_app_[a-z0-9_]+$`)

// TestNotification builds a test notification with two actions, as described in section 5 of the spec.
func TestNotification(l i18n.Lang, seq int64, clickAction string) ha.Notification {
	data := map[string]any{
		"tag": fmt.Sprintf("chores-test-%d", seq),
		"actions": []map[string]string{
			{"action": actions.Action{Kind: actions.Done, OccurrenceID: seq}.String(), "title": l.T("notif.done")},
			{"action": actions.Action{Kind: actions.Snooze, OccurrenceID: seq}.String(), "title": l.T("notif.snooze")},
		},
		"ttl":        0,
		"priority":   "high",
		"channel":    l.T("notif.channel"),
		"importance": "high", // only applied the first time the channel is used
	}
	if clickAction != "" {
		data["clickAction"] = clickAction // Android
		data["url"] = clickAction         // iOS
	}
	return ha.Notification{Title: l.T("notif.title"), Message: l.T("notif.test_message", seq), Data: data}
}

func (s *Server) handleTestNotify(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l := lang(w, r)
	service := strings.TrimPrefix(strings.TrimSpace(r.FormValue("service")), "notify.")
	click := strings.TrimSpace(r.FormValue("click_action"))
	if !serviceName.MatchString(service) {
		s.redirect(w, r, "", l.T("test.bad_service"), true)
		return
	}
	s.store.SetSetting(ctx, settingLastService, service)
	s.store.SetSetting(ctx, settingPanelPath, click)
	seq, err := s.store.NextSeq(ctx, "test_seq")
	if err != nil {
		s.redirect(w, r, "", l.T("test.db_error", err), true)
		return
	}
	n := TestNotification(l, seq, click)
	tag := n.Data["tag"].(string)
	payload, _ := json.Marshal(n)
	if err := s.client.Notify(ctx, service, n); err != nil {
		s.log.Error("sending test notification", "service", service, "err", err)
		s.store.LogEvent(ctx, store.Event{Kind: "error", Service: service, Tag: tag, Detail: err.Error()})
		s.redirect(w, r, "", l.T("test.send_failed", err), true)
		return
	}
	s.log.Info("test notification sent", "service", service, "tag", tag)
	s.store.LogEvent(ctx, store.Event{Kind: "notify", Service: service, Tag: tag, Detail: string(payload)})
	s.redirect(w, r, "", l.T("test.sent", seq, service), false)
}

// HandleAction handles a tap on a notification button (from Home Assistant or from the /dev page).
func (s *Server) HandleAction(ev ha.ActionEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	a, ok := actions.Parse(ev.Action)
	if !ok {
		s.log.Debug("ignoring foreign action", "action", ev.Action)
		return
	}
	s.log.Info("action tapped", "kind", a.Kind, "occurrence", a.OccurrenceID, "person", a.PersonID,
		"tag", ev.Tag, "ha_user_id", ev.UserID)
	detail := fmt.Sprintf("%s occurrence=%d person=%d ha_user_id=%q", a.Kind, a.OccurrenceID, a.PersonID, ev.UserID)
	if a.Kind == actions.Snooze {
		detail += " (snoozing arrives in M2)"
	}
	s.store.LogEvent(ctx, store.Event{Kind: "action", Tag: ev.Tag, Action: ev.Action, Detail: detail})

	if a.Kind != actions.Done || ev.Tag == "" {
		return
	}
	// After "Done" remove the notification with this tag (exercises clear_notification).
	service, ok, err := s.store.ServiceForTag(ctx, ev.Tag)
	if err != nil || !ok {
		return
	}
	if err := s.client.Notify(ctx, service, ha.Clear(ev.Tag)); err != nil {
		s.log.Error("clearing notification", "service", service, "tag", ev.Tag, "err", err)
		s.store.LogEvent(ctx, store.Event{Kind: "error", Service: service, Tag: ev.Tag, Detail: err.Error()})
		return
	}
	s.store.LogEvent(ctx, store.Event{Kind: "clear", Service: service, Tag: ev.Tag})
}

func (s *Server) handleDev(w http.ResponseWriter, r *http.Request) {
	d := s.basePage(w, r)
	d.Sent = s.fake.Sent()
	s.render(w, "dev", d)
}

var clickErrors = map[error]string{
	ha.ErrNoSuchNotification: "dev.err_missing",
	ha.ErrNotificationGone:   "dev.err_gone",
	ha.ErrNoSuchAction:       "dev.err_action",
	ha.ErrNotRunning:         "dev.err_not_ready",
}

func (s *Server) handleDevClick(w http.ResponseWriter, r *http.Request) {
	l := lang(w, r)
	id, _ := strconv.Atoi(r.FormValue("id"))
	action := r.FormValue("action")
	if err := s.fake.Click(id, action); err != nil {
		msg := err.Error()
		for e, key := range clickErrors {
			if errors.Is(err, e) {
				msg = l.T(key)
			}
		}
		s.redirect(w, r, "dev", msg, true)
		return
	}
	s.redirect(w, r, "dev", l.T("dev.clicked", action), false)
}
