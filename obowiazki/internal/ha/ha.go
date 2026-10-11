// Package ha talks to Home Assistant. The app logic only sees the Client interface,
// which has two implementations: Supervisor (real Home Assistant) and Fake (local, no Home Assistant).
package ha

import (
	"context"
	"encoding/json"
	"time"
)

// Notification is the service_data of a notify.mobile_app_* service call.
type Notification struct {
	Title   string         `json:"title,omitempty"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data,omitempty"`
}

// ClearMessage tells the Companion app to remove the notification with the given tag.
const ClearMessage = "clear_notification"

// Clear builds a message that removes the notification with the given tag.
func Clear(tag string) Notification {
	return Notification{Message: ClearMessage, Data: map[string]any{"tag": tag}}
}

// ActionEvent is the data of a mobile_app_notification_action event.
type ActionEvent struct {
	Action string
	Tag    string // Android sends back every data field of the notification, including tag
	UserID string // context.user_id, informational only
	At     time.Time
	Raw    json.RawMessage
}

// SelfInfo holds selected fields of /addons/self/info.
type SelfInfo struct {
	Slug       string `json:"slug"`
	IngressURL string `json:"ingress_url"`
}

type Client interface {
	Notify(ctx context.Context, service string, n Notification) error
	// NotifyServices returns notify.mobile_app_* service names (without the "notify." domain prefix).
	NotifyServices(ctx context.Context) ([]string, error)
	SelfInfo(ctx context.Context) (SelfInfo, error)
	// Run receives action taps and calls onAction until ctx is done.
	Run(ctx context.Context, onAction func(ActionEvent)) error
}
