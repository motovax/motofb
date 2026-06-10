package state

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strings"

	fberr "github.com/motovax/motofb/errors"
)

// FilesFromURLs downloads remote files for Mercury upload.
func FilesFromURLs(ctx context.Context, client *http.Client, urls []string) ([]FilePart, error) {
	if client == nil {
		client = http.DefaultClient
	}
	out := make([]FilePart, 0, len(urls))
	for _, fileURL := range urls {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fberr.Wrap("FilesFromURLs", "download file", err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fberr.Wrap("FilesFromURLs", fmt.Sprintf("status %d for %s", resp.StatusCode, fileURL), fberr.ErrNetwork)
		}
		filename := path.Base(fileURL)
		if i := strings.IndexAny(filename, "?#"); i >= 0 {
			filename = filename[:i]
		}
		if filename == "" || filename == "." || filename == "/" {
			filename = "file"
		}
		ct := resp.Header.Get("Content-Type")
		if ct == "" {
			ct = mime.TypeByExtension(path.Ext(filename))
		}
		if ct == "" {
			ct = "application/octet-stream"
		}
		out = append(out, FilePart{Filename: filename, Content: body, ContentType: ct})
	}
	return out, nil
}