// Package ha to warstwa rozmowy z Home Assistantem. Logika appki widzi tylko interfejs Client,
// który ma dwie implementacje: Supervisor (prawdziwy HA) i Fake (lokalnie, bez HA).
package ha

import (
	"context"
	"encoding/json"
	"time"
)

// Notification to service_data usługi notify.mobile_app_*.
type Notification struct {
	Title   string         `json:"title,omitempty"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data,omitempty"`
}

// ClearMessage to treść, która każe aplikacji Companion skasować powiadomienie o danym tagu.
const ClearMessage = "clear_notification"

// Clear buduje wiadomość kasującą powiadomienie o danym tagu.
func Clear(tag string) Notification {
	return Notification{Message: ClearMessage, Data: map[string]any{"tag": tag}}
}

// ActionEvent to dane zdarzenia mobile_app_notification_action.
type ActionEvent struct {
	Action string
	Tag    string // Android odsyła wszystkie pola data z powiadomienia, w tym tag
	UserID string // context.user_id, tylko informacyjnie
	At     time.Time
	Raw    json.RawMessage
}

// SelfInfo to wybrane pola z /addons/self/info.
type SelfInfo struct {
	Slug       string `json:"slug"`
	IngressURL string `json:"ingress_url"`
}

type Client interface {
	Notify(ctx context.Context, service string, n Notification) error
	// NotifyServices zwraca nazwy usług notify.mobile_app_* (bez prefiksu domeny "notify.").
	NotifyServices(ctx context.Context) ([]string, error)
	SelfInfo(ctx context.Context) (SelfInfo, error)
	// Run odbiera kliknięcia akcji i wywołuje onAction, aż ctx się skończy.
	Run(ctx context.Context, onAction func(ActionEvent)) error
}
