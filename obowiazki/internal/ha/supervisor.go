package ha

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// Supervisor rozmawia z HA przez proxy Supervisora: REST pod /core/api, WebSocket pod /core/websocket.
type Supervisor struct {
	baseURL string
	token   string
	http    *http.Client
	log     *slog.Logger
	// OnConnected jest wołane po każdym udanym (ponownym) połączeniu WebSocket.
	OnConnected func()
}

func NewSupervisor(baseURL, token string, log *slog.Logger) *Supervisor {
	return &Supervisor{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: 15 * time.Second},
		log:     log,
	}
}

func (s *Supervisor) do(ctx context.Context, method, path string, body any, out any) error {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, r)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("%s %s: HTTP %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}

func (s *Supervisor) Notify(ctx context.Context, service string, n Notification) error {
	return s.do(ctx, http.MethodPost, "/core/api/services/notify/"+service, n, nil)
}

func (s *Supervisor) NotifyServices(ctx context.Context) ([]string, error) {
	var domains []struct {
		Domain   string          `json:"domain"`
		Services json.RawMessage `json:"services"`
	}
	if err := s.do(ctx, http.MethodGet, "/core/api/services", nil, &domains); err != nil {
		return nil, err
	}
	var out []string
	for _, d := range domains {
		if d.Domain != "notify" {
			continue
		}
		// Docs pokazują listę nazw, a HA w praktyce zwraca obiekt nazwa → opis. Obsługujemy oba.
		var names []string
		if err := json.Unmarshal(d.Services, &names); err != nil {
			var m map[string]json.RawMessage
			if err := json.Unmarshal(d.Services, &m); err != nil {
				return nil, fmt.Errorf("nieznany format usług notify: %w", err)
			}
			for k := range m {
				names = append(names, k)
			}
		}
		for _, n := range names {
			if strings.HasPrefix(n, "mobile_app_") {
				out = append(out, n)
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

func (s *Supervisor) SelfInfo(ctx context.Context) (SelfInfo, error) {
	var resp struct {
		Data SelfInfo `json:"data"`
	}
	err := s.do(ctx, http.MethodGet, "/addons/self/info", nil, &resp)
	return resp.Data, err
}

// Run utrzymuje połączenie WebSocket i łączy się ponownie z narastającym opóźnieniem (1 s … 60 s).
func (s *Supervisor) Run(ctx context.Context, onAction func(ActionEvent)) error {
	backoff := time.Second
	for {
		start := time.Now()
		err := s.session(ctx, onAction)
		if ctx.Err() != nil {
			return nil
		}
		if time.Since(start) > time.Minute {
			backoff = time.Second // połączenie działało dłużej, więc zaczynamy od nowa
		}
		s.log.Warn("websocket: rozłączono, ponawiam", "err", err, "za", backoff)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, time.Minute)
	}
}

type wsMsg struct {
	ID      int64           `json:"id,omitempty"`
	Type    string          `json:"type"`
	Success *bool           `json:"success,omitempty"`
	Message string          `json:"message,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
	Event   json.RawMessage `json:"event,omitempty"`
}

func (s *Supervisor) session(ctx context.Context, onAction func(ActionEvent)) error {
	url := "ws" + strings.TrimPrefix(s.baseURL, "http") + "/core/websocket"
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return err
	}
	defer conn.CloseNow()
	conn.SetReadLimit(4 << 20)

	var m wsMsg
	if err := wsjson.Read(ctx, conn, &m); err != nil {
		return err
	}
	if m.Type != "auth_required" {
		return fmt.Errorf("oczekiwano auth_required, przyszło %q", m.Type)
	}
	if err := wsjson.Write(ctx, conn, map[string]string{"type": "auth", "access_token": s.token}); err != nil {
		return err
	}
	if err := wsjson.Read(ctx, conn, &m); err != nil {
		return err
	}
	if m.Type != "auth_ok" {
		return fmt.Errorf("uwierzytelnienie nieudane: %s %s", m.Type, m.Message)
	}

	const subID = 1
	if err := wsjson.Write(ctx, conn, map[string]any{
		"id": subID, "type": "subscribe_events", "event_type": "mobile_app_notification_action",
	}); err != nil {
		return err
	}
	s.log.Info("websocket: połączono, nasłuchuję mobile_app_notification_action")
	if s.OnConnected != nil {
		s.OnConnected()
	}
	for {
		m = wsMsg{}
		if err := wsjson.Read(ctx, conn, &m); err != nil {
			return err
		}
		switch {
		case m.Type == "result" && m.ID == subID:
			if m.Success == nil || !*m.Success {
				return fmt.Errorf("subscribe_events odrzucone: %s", m.Error)
			}
		case m.Type == "event" && m.ID == subID:
			ev, err := parseActionEvent(m.Event)
			if err != nil {
				s.log.Warn("websocket: nieczytelne zdarzenie", "err", err, "raw", string(m.Event))
				continue
			}
			onAction(ev)
		}
	}
}

func parseActionEvent(raw json.RawMessage) (ActionEvent, error) {
	var e struct {
		EventType string `json:"event_type"`
		Data      struct {
			Action string `json:"action"`
			Tag    string `json:"tag"`
		} `json:"data"`
		TimeFired time.Time `json:"time_fired"`
		Context   struct {
			UserID string `json:"user_id"`
		} `json:"context"`
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		return ActionEvent{}, err
	}
	if e.Data.Action == "" {
		return ActionEvent{}, errors.New("brak pola action")
	}
	return ActionEvent{Action: e.Data.Action, Tag: e.Data.Tag, UserID: e.Context.UserID, At: e.TimeFired, Raw: raw}, nil
}
