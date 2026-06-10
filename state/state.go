package state

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	fberr "github.com/motovax/motofb/errors"
	"github.com/motovax/motofb/graphql"
	"github.com/motovax/motofb/internal"
)

const defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36"

// State manages Facebook session credentials and HTTP transport.
type State struct {
	UserID   string
	UserName string
	Host     string

	FBDtsg       string
	FBDtsgAsync  string
	LSD          string
	Jazoest      string
	JazoestAsync string
	Revision     int
	MQTTClientID string
	MQTTAppID    string
	UserAppID    string
	Endpoint     string
	Region       string
	ClientID     string

	HTTP       *http.Client
	Jar        http.CookieJar
	UserAgent  string
	ReqCounter int
	LoggedIn   bool

	gql *graphql.Processor

	autoRefreshMu      sync.Mutex
	autoRefreshEnabled bool
	refreshInterval    time.Duration
	autoRefreshStop    chan struct{}
}

// Options configures State construction.
type Options struct {
	UserAgent string
	ProxyURL  string
	HTTP      *http.Client
}

// FromCookieFile loads cookies from path and authenticates against facebook.com.
func FromCookieFile(ctx context.Context, path string, opts Options) (*State, error) {
	jar, err := LoadCookiesFromFile(path)
	if err != nil {
		return nil, err
	}
	return Login(ctx, jar, opts)
}

// FromCookieRecords restores state from serialized cookie records.
func FromCookieRecords(ctx context.Context, records []CookieRecord, opts Options) (*State, error) {
	jar, err := CookiesToJar(records)
	if err != nil {
		return nil, err
	}
	return Login(ctx, jar, opts)
}

// Login validates cookies and extracts tokens from Facebook HTML.
func Login(ctx context.Context, jar http.CookieJar, opts Options) (*State, error) {
	userID, err := UserIDFromJar(jar)
	if err != nil {
		return nil, err
	}

	ua := opts.UserAgent
	if ua == "" {
		ua = defaultUserAgent
	}

	client := opts.HTTP
	if client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		if opts.ProxyURL != "" {
			proxyURL, err := url.Parse(opts.ProxyURL)
			if err != nil {
				return nil, fberr.Wrap("Login", "parse proxy url", err)
			}
			transport.Proxy = http.ProxyURL(proxyURL)
		}
		client = &http.Client{
			Timeout:   60 * time.Second,
			Jar:       jar,
			Transport: transport,
		}
	} else if client.Jar == nil {
		client.Jar = jar
	}

	host := "www.facebook.com"
	pageURL := "https://www.facebook.com/"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, fberr.Wrap("Login", "build request", err)
	}
	setGETHeaders(req, host, ua)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fberr.Wrap("Login", "fetch facebook", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		loc := resp.Header.Get("Location")
		if loc != "" {
			if u, err := url.Parse(loc); err == nil && u.Host != "" {
				host = u.Host
				pageURL = fmt.Sprintf("https://%s/", host)
			}
			_ = resp.Body.Close()
			req, err = http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
			if err != nil {
				return nil, fberr.Wrap("Login", "build redirect request", err)
			}
			setGETHeaders(req, host, ua)
			resp, err = client.Do(req)
			if err != nil {
				return nil, fberr.Wrap("Login", "follow redirect", err)
			}
			defer resp.Body.Close()
		}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fberr.Wrap("Login", fmt.Sprintf("unexpected status %d", resp.StatusCode), fberr.ErrNetwork)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fberr.Wrap("Login", "read response", err)
	}

	tokens, err := ExtractTokensFromHTML(string(body))
	if err != nil {
		return nil, err
	}

	return &State{
		UserID:       userID,
		UserName:     tokens.UserName,
		Host:         host,
		FBDtsg:       tokens.FBDtsg,
		FBDtsgAsync:  tokens.FBDtsgAsync,
		LSD:          tokens.LSD,
		Jazoest:      tokens.Jazoest,
		JazoestAsync: tokens.JazoestAsync,
		Revision:     tokens.ClientRevision,
		MQTTClientID: tokens.MQTTClientID,
		MQTTAppID:    tokens.MQTTAppID,
		UserAppID:    tokens.UserAppID,
		Endpoint:     tokens.Endpoint,
		Region:       tokens.Region,
		ClientID:     internal.GenerateClientID(),
		HTTP:         client,
		Jar:          jar,
		UserAgent:    ua,
		LoggedIn:     true,
		gql:          graphql.NewProcessor(),
	}, nil
}

// Snapshot exports cookies for persistence.
func (s *State) Snapshot() map[string]any {
	return map[string]any{
		"version": 1,
		"cookies": DumpCookies(s.Jar),
	}
}

// NextReqParams returns standard Facebook POST/GET parameters.
func (s *State) NextReqParams() map[string]string {
	s.ReqCounter++
	return map[string]string{
		"__user":  s.UserID,
		"__a":     "1",
		"__req":   internal.DecimalToBase36(s.ReqCounter),
		"__rev":   fmt.Sprintf("%d", s.Revision),
		"fb_dtsg": s.FBDtsg,
		"jazoest": s.Jazoest,
	}
}

// BuildHeaders constructs request headers for a given URL and method class.
func (s *State) BuildHeaders(rawURL, requestType string, graphqlFriendlyName string) http.Header {
	u, _ := url.Parse(rawURL)
	host := u.Host
	if host == "" {
		host = s.Host
	}
	base := "https://" + host

	h := http.Header{}
	h.Set("User-Agent", s.UserAgent)
	h.Set("Accept-Language", "en-US,en;q=0.9")
	h.Set("Accept-Encoding", "gzip, deflate, br")
	h.Set("Host", host)
	h.Set("Origin", base)
	h.Set("Referer", base+"/")

	switch requestType {
	case "get":
		h.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	case "post":
		h.Set("Accept", "*/*")
		h.Set("Content-Type", "application/x-www-form-urlencoded")
	case "upload":
		h.Set("Accept", "*/*")
	}

	if strings.Contains(host, "messenger.com") {
		h.Set("Origin", "https://www.messenger.com")
		h.Set("Referer", "https://www.messenger.com/")
	}
	if strings.HasPrefix(host, "m.") {
		h.Set("Origin", "https://m.facebook.com")
		h.Set("Referer", "https://m.facebook.com/")
	}
	if graphqlFriendlyName != "" {
		h.Set("X-Fb-Friendly-Name", graphqlFriendlyName)
		h.Set("X-Fb-Lsd", s.LSD)
	}
	return h
}

// Get performs an authenticated GET and decodes JSON.
func (s *State) Get(ctx context.Context, rawURL string, params map[string]string) (map[string]any, error) {
	raw, err := s.withRetry(ctx, 3, func() (any, error) {
		return s.getOnce(ctx, rawURL, params)
	})
	if err != nil {
		return nil, err
	}
	return raw.(map[string]any), nil
}

func (s *State) getOnce(ctx context.Context, rawURL string, params map[string]string) (map[string]any, error) {
	fullURL := internal.PrefixURL(rawURL, s.Host)
	q := s.NextReqParams()
	for k, v := range params {
		q[k] = v
	}

	u, err := url.Parse(fullURL)
	if err != nil {
		return nil, err
	}
	uv := u.Query()
	for k, v := range q {
		uv.Set(k, v)
	}
	u.RawQuery = uv.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	for k, vals := range s.BuildHeaders(fullURL, "get", "") {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}

	resp, err := s.HTTP.Do(req)
	if err != nil {
		return nil, fberr.Wrap("State.Get", "http get", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fberr.Wrap("State.Get", fmt.Sprintf("status %d", resp.StatusCode), fberr.ErrNetwork)
	}

	out, err := s.gql.ProcessNormalResponse(string(body))
	if err != nil {
		return nil, err
	}
	if err := s.gql.HandlePayloadError(out); err != nil {
		return nil, err
	}
	return out, nil
}

// Post performs an authenticated POST.
func (s *State) Post(ctx context.Context, rawURL string, data map[string]string, asGraphQL bool) (any, error) {
	return s.withRetry(ctx, 3, func() (any, error) {
		return s.postOnce(ctx, rawURL, data, asGraphQL)
	})
}

func (s *State) postOnce(ctx context.Context, rawURL string, data map[string]string, asGraphQL bool) (any, error) {
	fullURL := internal.PrefixURL(rawURL, s.Host)
	params := s.NextReqParams()
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	for k, v := range data {
		form.Set(k, v)
	}

	friendly := data["fb_api_req_friendly_name"]
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	for k, vals := range s.BuildHeaders(fullURL, "post", friendly) {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}

	resp, err := s.HTTP.Do(req)
	if err != nil {
		return nil, fberr.Wrap("State.Post", "http post", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fberr.Wrap("State.Post", fmt.Sprintf("status %d", resp.StatusCode), fberr.ErrNetwork)
	}

	content := string(body)
	if asGraphQL {
		return s.gql.ProcessResponse(content)
	}
	out, err := s.gql.ProcessNormalResponse(content)
	if err != nil {
		return nil, err
	}
	if err := s.gql.HandlePayloadError(out); err != nil {
		return nil, err
	}
	return out, nil
}

// GraphQLBatch executes one or more doc_id queries.
func (s *State) GraphQLBatch(ctx context.Context, queries ...graphql.QueryRequest) ([]map[string]any, error) {
	return s.GraphQLBatchNamed(ctx, "", queries...)
}

// GraphQLBatchNamed executes doc_id queries with an optional batch_name.
func (s *State) GraphQLBatchNamed(ctx context.Context, batchName string, queries ...graphql.QueryRequest) ([]map[string]any, error) {
	data := map[string]string{
		"queries": s.gql.QueriesToJSON(queries...),
	}
	if batchName != "" {
		data["batch_name"] = batchName
	}
	raw, err := s.Post(ctx, "https://www.facebook.com/api/graphqlbatch/", data, true)
	if err != nil {
		return nil, err
	}
	slice, ok := raw.([]map[string]any)
	if !ok {
		return nil, fberr.New("State.GraphQLBatch", "unexpected graphql response type")
	}
	return slice, nil
}

// Refresh re-fetches tokens using the existing cookie jar.
func (s *State) Refresh(ctx context.Context) error {
	next, err := Login(ctx, s.Jar, Options{UserAgent: s.UserAgent, HTTP: s.HTTP})
	if err != nil {
		s.LoggedIn = false
		return err
	}
	s.UserID = next.UserID
	s.UserName = next.UserName
	s.FBDtsg = next.FBDtsg
	s.FBDtsgAsync = next.FBDtsgAsync
	s.LSD = next.LSD
	s.Jazoest = next.Jazoest
	s.JazoestAsync = next.JazoestAsync
	s.Revision = next.Revision
	s.MQTTClientID = next.MQTTClientID
	s.MQTTAppID = next.MQTTAppID
	s.UserAppID = next.UserAppID
	s.Endpoint = next.Endpoint
	s.Region = next.Region
	s.LoggedIn = true
	return nil
}

// Close releases HTTP resources. The default client needs no action.
func (s *State) Close() error {
	s.DisableAutoRefresh()
	s.LoggedIn = false
	return nil
}

func setGETHeaders(req *http.Request, host, ua string) {
	base := "https://" + host
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Host", host)
	req.Header.Set("Origin", base)
	req.Header.Set("Referer", base+"/")
}