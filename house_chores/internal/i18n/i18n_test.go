package i18n

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCatalogsHaveSameKeys(t *testing.T) {
	for k := range catalog[EN] {
		if _, ok := catalog[PL][k]; !ok {
			t.Errorf("pl missing %q", k)
		}
	}
	for k := range catalog[PL] {
		if _, ok := catalog[EN][k]; !ok {
			t.Errorf("en missing %q", k)
		}
	}
}

func TestFromRequest(t *testing.T) {
	cases := []struct {
		query, cookie, accept string
		want                  Lang
	}{
		{"", "", "", EN},
		{"", "", "pl-PL,pl;q=0.9,en;q=0.8", PL},
		{"", "", "de-DE,de;q=0.9", EN},
		{"", "", "en-US,en;q=0.9", EN},
		{"", "pl", "en-US", PL},
		{"en", "pl", "pl-PL", EN},
		{"xx", "", "pl", PL},
	}
	for _, c := range cases {
		r := httptest.NewRequest("GET", "/?lang="+c.query, nil)
		if c.cookie != "" {
			r.AddCookie(&http.Cookie{Name: CookieName, Value: c.cookie})
		}
		if c.accept != "" {
			r.Header.Set("Accept-Language", c.accept)
		}
		if got := FromRequest(r); got != c.want {
			t.Errorf("%+v: got %s", c, got)
		}
	}
}

func TestT(t *testing.T) {
	if got := PL.T("test.sent", 3, "mobile_app_x"); got != "Wysłano testowe powiadomienie #3 na notify.mobile_app_x" {
		t.Error(got)
	}
	if got := EN.T("no.such.key"); got != "no.such.key" {
		t.Error(got)
	}
}
