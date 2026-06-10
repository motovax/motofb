package state

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	fberr "github.com/motovax/motofb/errors"
)

// PostRaw returns raw response bytes (for user_info endpoints).
func (s *State) PostRaw(ctx context.Context, rawURL string, data map[string]string) ([]byte, error) {
	fullURL := internalPrefix(rawURL, s.Host)
	params := s.NextReqParams()
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	for k, v := range data {
		form.Set(k, v)
	}
	req, err := httpNewPost(ctx, fullURL, form.Encode(), s)
	if err != nil {
		return nil, err
	}
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fberr.Wrap("PostRaw", fmt.Sprintf("status %d", resp.StatusCode), fberr.ErrNetwork)
	}
	return body, nil
}

// PostGraphQLSingle posts to /api/graphql/ and returns raw bytes.
func (s *State) PostGraphQLSingle(ctx context.Context, data map[string]string) ([]byte, error) {
	return s.PostRaw(ctx, "https://www.facebook.com/api/graphql/", data)
}

// PostNoResponse fires a GraphQL mutation without reading body.
func (s *State) PostNoResponse(ctx context.Context, rawURL string, data map[string]string) error {
	_, err := s.PostRaw(ctx, rawURL, data)
	return err
}

func internalPrefix(url, host string) string {
	if strings.HasPrefix(url, "http") {
		return url
	}
	if strings.HasPrefix(url, "/") {
		return "https://" + host + url
	}
	return url
}

// import cycle avoid - duplicate small helper
func httpNewPost(ctx context.Context, fullURL, body string, s *State) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	friendly := ""
	for k, v := range parseForm(body) {
		if k == "fb_api_req_friendly_name" {
			friendly = v
		}
	}
	for k, vals := range s.BuildHeaders(fullURL, "post", friendly) {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
	return req, nil
}

func parseForm(body string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(body, "&") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			out[kv[0]] = kv[1]
		}
	}
	return out
}