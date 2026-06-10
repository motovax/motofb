package state

import (
	"context"
	"io"
	"net/http"
	"os"
)

// DownloadFile saves a URL to a local file.
func (s *State) DownloadFile(ctx context.Context, fileURL, filename string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}