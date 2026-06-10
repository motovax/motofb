package state_test

import (
	"testing"

	"github.com/motovax/motofb/state"
)

func TestCookiesToJarAndUserID(t *testing.T) {
	records := []state.CookieRecord{
		{Name: "c_user", Value: "100001", Path: "/"},
		{Name: "xs", Value: "secret", Path: "/"},
	}
	jar, err := state.CookiesToJar(records)
	if err != nil {
		t.Fatal(err)
	}
	uid, err := state.UserIDFromJar(jar)
	if err != nil {
		t.Fatal(err)
	}
	if uid != "100001" {
		t.Fatalf("uid = %q", uid)
	}

	dump := state.DumpCookies(jar)
	if len(dump) == 0 {
		t.Fatal("expected dumped cookies")
	}
}

func TestLoadCookiesFromJSON(t *testing.T) {
	data := []byte(`[
		{"name":"c_user","value":"42","path":"/"},
		{"name":"xs","value":"tok","path":"/"}
	]`)
	jar, err := state.LoadCookiesFromJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	uid, err := state.UserIDFromJar(jar)
	if err != nil || uid != "42" {
		t.Fatalf("uid=%q err=%v", uid, err)
	}
}

func TestCookieHeader(t *testing.T) {
	records := []state.CookieRecord{{Name: "c_user", Value: "1", Path: "/"}}
	jar, err := state.CookiesToJar(records)
	if err != nil {
		t.Fatal(err)
	}
	header, err := state.CookieHeader(jar, "https://www.facebook.com")
	if err != nil {
		t.Fatal(err)
	}
	if header == "" {
		t.Fatal("expected cookie header")
	}
}