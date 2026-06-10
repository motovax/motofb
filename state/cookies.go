package state

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/net/publicsuffix"

	fberr "github.com/motovax/motofb/errors"
)

// CookieRecord is a browser-exported cookie entry (fbstate / C3C format).
type CookieRecord struct {
	Name    string `json:"name"`
	Key     string `json:"key"`
	Value   string `json:"value"`
	Path    string `json:"path"`
	Expires any    `json:"expires"`
}

var cookieTargets = []struct {
	URL    string
	Domain string
}{
	{"https://www.facebook.com", ".facebook.com"},
	{"https://www.messenger.com", ".messenger.com"},
	{"https://rupload-ccu1-2.up.facebook.com", "up.facebook.com"},
}

// LoadCookiesFromFile reads a JSON cookie export and populates a jar.
func LoadCookiesFromFile(path string) (http.CookieJar, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fberr.Wrap("LoadCookiesFromFile", "read cookies file", err)
	}
	return LoadCookiesFromJSON(data)
}

// LoadCookiesFromJSON parses cookie JSON bytes into a jar.
func LoadCookiesFromJSON(data []byte) (http.CookieJar, error) {
	var records []CookieRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fberr.Wrap("LoadCookiesFromJSON", "decode cookies", err)
	}
	if len(records) == 0 {
		return nil, fberr.New("LoadCookiesFromJSON", "expected a non-empty list of cookie objects")
	}
	return CookiesToJar(records)
}

// CookiesToJar builds a cookie jar from fbstate-compatible records.
func CookiesToJar(records []CookieRecord) (http.CookieJar, error) {
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, fberr.Wrap("CookiesToJar", "create cookie jar", err)
	}

	keyField := "name"
	if records[0].Key != "" {
		keyField = "key"
	}

	for _, rec := range records {
		name := rec.Name
		if keyField == "key" {
			name = rec.Key
		}
		if name == "" || rec.Value == "" {
			continue
		}
		path := rec.Path
		if path == "" {
			path = "/"
		}

		for _, target := range cookieTargets {
			u, err := url.Parse(target.URL)
			if err != nil {
				return nil, fberr.Wrap("CookiesToJar", "parse target url", err)
			}
			c := &http.Cookie{
				Name:   name,
				Value:  rec.Value,
				Path:   path,
				Domain: target.Domain,
			}
			if rec.Expires != nil {
				switch v := rec.Expires.(type) {
				case float64:
					c.Expires = time.Unix(int64(v), 0)
				case string:
					if t, err := time.Parse(time.RFC3339, v); err == nil {
						c.Expires = t
					}
				}
			}
			jar.SetCookies(u, []*http.Cookie{c})
		}
	}
	return jar, nil
}

// DumpCookies serializes jar cookies to fbstate-compatible records.
func DumpCookies(jar http.CookieJar) []map[string]any {
	seen := make(map[string]struct{})
	var out []map[string]any

	for _, target := range cookieTargets {
		u, _ := url.Parse(target.URL)
		for _, c := range jar.Cookies(u) {
			key := c.Name + "|" + c.Value + "|" + c.Domain
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			item := map[string]any{
				"name":  c.Name,
				"value": c.Value,
				"path":  c.Path,
			}
			if !c.Expires.IsZero() {
				item["expires"] = c.Expires.Unix()
			}
			out = append(out, item)
		}
	}
	return out
}

// UserIDFromJar reads c_user from facebook.com cookies.
func UserIDFromJar(jar http.CookieJar) (string, error) {
	u, _ := url.Parse("https://www.facebook.com")
	for _, c := range jar.Cookies(u) {
		if c.Name == "c_user" {
			if c.Value == "" {
				break
			}
			return c.Value, nil
		}
	}
	return "", fberr.Wrap("UserIDFromJar", "c_user cookie not found", fberr.ErrAuthentication)
}

// CookieHeader returns a Cookie header string for the given base URL.
func CookieHeader(jar http.CookieJar, baseURL string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	parts := make([]string, 0, 8)
	for _, c := range jar.Cookies(u) {
		parts = append(parts, fmt.Sprintf("%s=%s", c.Name, c.Value))
	}
	return strings.Join(parts, "; "), nil
}