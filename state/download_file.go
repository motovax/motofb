package state

import (
	"context"
	"io"
	"net/http"
	"os"
)

// DownloadFile saves a URL to a local file using the authenticated session.
func (s *State) DownloadFile(ctx context.Context, fileURL, filename string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return err
	}
	for k, vals := range s.BuildHeaders(fileURL, "get", "") {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
	client := s.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	buf := make([]byte, 64*1024)
	_, err = io.CopyBuffer(f, resp.Body, buf)
	return err
}