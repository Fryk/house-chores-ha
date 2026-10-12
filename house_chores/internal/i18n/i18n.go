// Package i18n holds the UI and notification strings in Polish and English.
package i18n

import (
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/text/language"
)

type Lang string

const (
	PL Lang = "pl"
	EN Lang = "en"
)

// Default is used when neither the cookie nor Accept-Language picks a supported language.
const Default = EN

var supported = []language.Tag{language.English, language.Polish}
var matcher = language.NewMatcher(supported)

// CookieName stores the language chosen with the switcher in the page header.
const CookieName = "lang"

// Parse returns a supported language or ok=false.
func Parse(s string) (Lang, bool) {
	switch Lang(strings.ToLower(strings.TrimSpace(s))) {
	case PL:
		return PL, true
	case EN:
		return EN, true
	}
	return "", false
}

// FromRequest picks the language: ?lang= query, then cookie, then Accept-Language.
func FromRequest(r *http.Request) Lang {
	if l, ok := Parse(r.URL.Query().Get("lang")); ok {
		return l
	}
	if c, err := r.Cookie(CookieName); err == nil {
		if l, ok := Parse(c.Value); ok {
			return l
		}
	}
	if al := r.Header.Get("Accept-Language"); al != "" {
		tags, _, err := language.ParseAcceptLanguage(al)
		if err == nil && len(tags) > 0 {
			_, idx, conf := matcher.Match(tags...)
			if conf != language.No {
				if supported[idx] == language.Polish {
					return PL
				}
				return EN
			}
		}
	}
	return Default
}

// T returns the translation of key, formatted with args. Missing keys fall back to English, then to the key.
func (l Lang) T(key string, args ...any) string {
	s, ok := catalog[l][key]
	if !ok {
		s, ok = catalog[EN][key]
	}
	if !ok {
		s = key
	}
	if len(args) > 0 {
		return fmt.Sprintf(s, args...)
	}
	return s
}

var catalog = map[Lang]map[string]string{
	EN: {
		"app.name":           "Chores",
		"mode":               "%s mode",
		"nav.home":           "Home",
		"nav.lang_other":     "Polski",
		"test.title":         "Test notification",
		"test.service":       "Notify service (phone)",
		"test.no_services":   "No notify.mobile_app_* services. Install the Home Assistant Companion app on your phone and allow notifications.",
		"test.services_err":  "Could not load services from Home Assistant: %s",
		"test.click_action":  "Path opened when the notification is tapped (clickAction)",
		"test.click_help":    "Default: %s. The Home Assistant docs do not specify the app panel path: open the app panel, check the address and enter it here. The value is saved.",
		"test.send":          "Send test notification",
		"test.bad_service":   "Enter a service in the form mobile_app_…",
		"test.db_error":      "Database error: %s",
		"test.send_failed":   "Sending failed: %s",
		"test.sent":          "Test notification #%d sent to notify.%s",
		"events.title":       "Recent events",
		"events.empty":       "Nothing has happened yet.",
		"diag.title":         "Ingress diagnostics",
		"diag.slug":          "App slug",
		"diag.ingress_url":   "Ingress URL",
		"diag.schema":        "Database schema version",
		"dev.phones":         "Notifications on simulated phones",
		"dev.phones_help":    "This is how they would look on the phones. Tapping a button sends the app a mobile_app_notification_action event, the same as from Android.",
		"dev.refresh":        "Refresh",
		"dev.no_title":       "(no title)",
		"dev.replaced":       "replaced by a newer one",
		"dev.cleared":        "cleared",
		"dev.empty":          "No notifications. Send a test one from the home page.",
		"dev.devices":        "Appliances",
		"dev.devices_help":   "Simulating washer, dryer and dishwasher cycle start and end comes in M4.",
		"dev.washer_start":   "Washer: cycle start",
		"dev.washer_end":     "Washer: cycle end",
		"dev.clock":          "Clock",
		"dev.clock_help":     "Shifting the clock to test schedules comes with the scheduler in M2. Now: %s.",
		"dev.plus_hour":      "+1 hour",
		"dev.plus_day":       "+1 day",
		"dev.clicked":        "Tapped %s; details in the log and on the home page",
		"dev.err_missing":    "No such notification",
		"dev.err_gone":       "This notification is no longer on the phone",
		"dev.err_action":     "The notification has no such action",
		"dev.err_not_ready":  "Event handling is not running yet",
		"notif.title":        "Chores",
		"notif.test_message": "Test notification #%d",
		"notif.done":         "Done",
		"notif.snooze":       "In 1 hour",
		"notif.channel":      "Chores",
	},
	PL: {
		"app.name":           "Obowiązki",
		"mode":               "tryb %s",
		"nav.home":           "Start",
		"nav.lang_other":     "English",
		"test.title":         "Testowe powiadomienie",
		"test.service":       "Usługa notify (telefon)",
		"test.no_services":   "Brak usług notify.mobile_app_*. Zainstaluj aplikację Home Assistant na telefonie i zezwól na powiadomienia.",
		"test.services_err":  "Nie udało się pobrać usług z Home Assistanta: %s",
		"test.click_action":  "Ścieżka otwierana po tapnięciu w treść (clickAction)",
		"test.click_help":    "Domyślnie %s. Dokumentacja Home Assistanta nie podaje ścieżki panelu appki: otwórz panel appki, sprawdź adres i wpisz go tutaj. Wartość zostanie zapamiętana.",
		"test.send":          "Wyślij testowe powiadomienie",
		"test.bad_service":   "Podaj usługę w postaci mobile_app_…",
		"test.db_error":      "Błąd bazy: %s",
		"test.send_failed":   "Nie udało się wysłać: %s",
		"test.sent":          "Wysłano testowe powiadomienie #%d na notify.%s",
		"events.title":       "Ostatnie zdarzenia",
		"events.empty":       "Jeszcze nic się nie wydarzyło.",
		"diag.title":         "Diagnostyka ingress",
		"diag.slug":          "Slug appki",
		"diag.ingress_url":   "Ingress URL",
		"diag.schema":        "Wersja schematu bazy",
		"dev.phones":         "Powiadomienia na udawanych telefonach",
		"dev.phones_help":    "Tak wyglądałyby na telefonach. Kliknięcie przycisku wysyła do appki zdarzenie mobile_app_notification_action, takie jak z Androida.",
		"dev.refresh":        "Odśwież",
		"dev.no_title":       "(bez tytułu)",
		"dev.replaced":       "zastąpione nowszym",
		"dev.cleared":        "skasowane",
		"dev.empty":          "Brak powiadomień. Wyślij testowe ze strony głównej.",
		"dev.devices":        "Urządzenia",
		"dev.devices_help":   "Symulacja startu i końca cyklu pralki, suszarki i zmywarki pojawi się w M4.",
		"dev.washer_start":   "Pralka: start cyklu",
		"dev.washer_end":     "Pralka: koniec cyklu",
		"dev.clock":          "Zegar",
		"dev.clock_help":     "Przesuwanie zegara do testów harmonogramów pojawi się razem ze schedulerem w M2. Teraz: %s.",
		"dev.plus_hour":      "+1 godzina",
		"dev.plus_day":       "+1 dzień",
		"dev.clicked":        "Kliknięto %s; szczegóły w logu i na stronie głównej",
		"dev.err_missing":    "Nie ma takiego powiadomienia",
		"dev.err_gone":       "Tego powiadomienia nie ma już na telefonie",
		"dev.err_action":     "Powiadomienie nie ma takiej akcji",
		"dev.err_not_ready":  "Odbiór zdarzeń jeszcze nie działa",
		"notif.title":        "Obowiązki",
		"notif.test_message": "Testowe powiadomienie #%d",
		"notif.done":         "Zrobione",
		"notif.snooze":       "Za godzinę",
		"notif.channel":      "Obowiązki",
	},
}
