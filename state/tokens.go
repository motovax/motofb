package state

import (
	"regexp"
	"strconv"
	"strings"

	fberr "github.com/motovax/motofb/errors"
	"github.com/motovax/motofb/internal"
)

// Tokens holds authentication values scraped from Facebook HTML.
type Tokens struct {
	FBDtsg         string
	FBDtsgAsync    string
	LSD            string
	Jazoest        string
	JazoestAsync   string
	ClientRevision int
	MQTTClientID   string
	MQTTAppID      string
	UserAppID      string
	Endpoint       string
	Region         string
	UserName       string
}

var (
	reDTSG         = regexp.MustCompile(`"DTSGInitialData".*?"token":"(.*?)"`)
	reDTSGAsync    = regexp.MustCompile(`"DTSGInitData"(?:\s*,\s*\[\])?(?:\s*,\s*)\{[^}]*"async_get_token"\s*:\s*"([^"]+)"[^}]*\}`)
	reLSD          = regexp.MustCompile(`"LSD"\s*,\s*\[\s*\]\s*,\s*\{\s*"token"\s*:\s*"([A-Za-z0-9_-]+)"`)
	reClientRev    = regexp.MustCompile(`client_revision":(\d+)`)
	reMQTTClientID = regexp.MustCompile(`\["MqttWebDeviceID".*?"clientID"\s*:\s*"([a-f0-9\-]+)"`)
	reMQTTAppID    = regexp.MustCompile(`\["MqttWebConfig".*?"appID"\s*:\s*(\d+)`)
	reUserAppID    = regexp.MustCompile(`\["CurrentUserInitialData".*?"APP_ID"\s*:\s*"(\d+)"`)
	reMQTTEndpoint = regexp.MustCompile(`"endpoint"\s*:\s*"([^"]*?region=([a-zA-Z0-9_-]+)[^"]*)"`)
	reUserName     = regexp.MustCompile(`"NAME"\s*:\s*"([^"]+)"`)
)

// ExtractTokensFromHTML parses login tokens from a facebook.com HTML response.
func ExtractTokensFromHTML(html string) (Tokens, error) {
	var t Tokens

	m := reDTSG.FindStringSubmatch(html)
	if len(m) < 2 {
		return t, fberr.Wrap("ExtractTokensFromHTML", "fb_dtsg token not found", fberr.ErrValidation)
	}
	t.FBDtsg = m[1]

	m = reDTSGAsync.FindStringSubmatch(html)
	if len(m) < 2 {
		return t, fberr.Wrap("ExtractTokensFromHTML", "async_get_token not found", fberr.ErrValidation)
	}
	t.FBDtsgAsync = m[1]

	if m = reLSD.FindStringSubmatch(html); len(m) >= 2 {
		t.LSD = m[1]
	}

	t.Jazoest = internal.Jazoest(t.FBDtsg)
	t.JazoestAsync = internal.Jazoest(t.FBDtsgAsync)

	if m = reClientRev.FindStringSubmatch(html); len(m) >= 2 {
		rev, _ := strconv.Atoi(m[1])
		t.ClientRevision = rev
	}
	if m = reMQTTClientID.FindStringSubmatch(html); len(m) >= 2 {
		t.MQTTClientID = m[1]
	}
	if m = reMQTTAppID.FindStringSubmatch(html); len(m) >= 2 {
		t.MQTTAppID = m[1]
	}
	if m = reUserAppID.FindStringSubmatch(html); len(m) >= 2 {
		t.UserAppID = m[1]
	}
	if m = reMQTTEndpoint.FindStringSubmatch(html); len(m) >= 3 {
		t.Endpoint = unescapeJSONString(m[1])
		t.Region = m[2]
	} else {
		return t, fberr.Wrap("ExtractTokensFromHTML", "mqtt endpoint not found", fberr.ErrValidation)
	}
	if m = reUserName.FindStringSubmatch(html); len(m) >= 2 {
		t.UserName = m[1]
	}
	return t, nil
}

func unescapeJSONString(s string) string {
	s = strings.ReplaceAll(s, `\/`, `/`)
	s = strings.ReplaceAll(s, `\u0026`, `&`)
	return s
}