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

// Supervisor talks to Home Assistant through the Supervisor proxy: REST at /core/api, WebSocket at /core/websocket.
type Supervisor struct {
	baseURL string
	token   string
	http    *http.Client
	log     *slog.Logger
	// OnConnected is called after every successful (re)connection of the WebSocket.
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
		// The docs show a list of names; Home Assistant actually returns a name → description object. Accept both.
		var names []string
		if err := json.Unmarshal(d.Services, &names); err != nil {
			var m map[string]json.RawMessage
			if err := json.Unmarshal(d.Services, &m); err != nil {
				return nil, fmt.Errorf("unknown notify services format: %w", err)
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

// Run keeps the WebSocket connected, reconnecting with exponential backoff (1 s … 60 s).
func (s *Supervisor) Run(ctx context.Context, onAction func(ActionEvent)) error {
	backoff := time.Second
	for {
		start := time.Now()
		err := s.session(ctx, onAction)
		if ctx.Err() != nil {
			return nil
		}
		if time.Since(start) > time.Minute {
			backoff = time.Second // the connection was healthy for a while, start over
		}
		s.log.Warn("websocket: disconnected, retrying", "err", err, "in", backoff)
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
		return fmt.Errorf("expected auth_required, got %q", m.Type)
	}
	if err := wsjson.Write(ctx, conn, map[string]string{"type": "auth", "access_token": s.token}); err != nil {
		return err
	}
	if err := wsjson.Read(ctx, conn, &m); err != nil {
		return err
	}
	if m.Type != "auth_ok" {
		return fmt.Errorf("authentication failed: %s %s", m.Type, m.Message)
	}

	const subID = 1
	if err := wsjson.Write(ctx, conn, map[string]any{
		"id": subID, "type": "subscribe_events", "event_type": "mobile_app_notification_action",
	}); err != nil {
		return err
	}
	s.log.Info("websocket: connected, listening for mobile_app_notification_action")
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
				return fmt.Errorf("subscribe_events rejected: %s", m.Error)
			}
		case m.Type == "event" && m.ID == subID:
			ev, err := parseActionEvent(m.Event)
			if err != nil {
				s.log.Warn("websocket: unreadable event", "err", err, "raw", string(m.Event))
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
		return ActionEvent{}, errors.New("missing action field")
	}
	return ActionEvent{Action: e.Data.Action, Tag: e.Data.Tag, UserID: e.Context.UserID, At: e.TimeFired, Raw: raw}, nil
}
