package state

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	fberr "github.com/motovax/motofb/errors"
	"github.com/motovax/motofb/internal"
)

// FilePart is one upload file.
type FilePart struct {
	Filename    string
	Content     []byte
	ContentType string
}

// UploadFiles uploads attachments to Mercury and returns file ids.
func (s *State) UploadFiles(ctx context.Context, files []FilePart, voiceClip bool) ([]int64, error) {
	if len(files) == 0 {
		return nil, fberr.New("UploadFiles", "no files provided")
	}
	body := &bytes.Buffer{}
	writer := multipartWriter(body)
	params := s.NextReqParams()
	for k, v := range params {
		_ = writer.WriteField(k, v)
	}
	if voiceClip {
		_ = writer.WriteField("voice_clip", "true")
	} else {
		_ = writer.WriteField("voice_clip", "false")
	}
	for i, f := range files {
		ct := f.ContentType
		if ct == "" {
			ct = "application/octet-stream"
		}
		if err := writer.WriteFile(fmt.Sprintf("upload_%d", i), f.Filename, ct, f.Content); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	url := "https://upload.facebook.com/ajax/mercury/upload.php"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	for k, vals := range s.BuildHeaders(url, "upload", "") {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	out, err := s.gql.ProcessNormalResponse(string(raw))
	if err != nil {
		return nil, err
	}
	payload, _ := out["payload"].(map[string]any)
	meta, _ := payload["metadata"].(map[string]any)
	ids := make([]int64, 0, len(files))
	for _, v := range meta {
		m, _ := v.(map[string]any)
		ft, _ := m["filetype"].(string)
		key := internal.MIMEToKey(ft)
		id := int64(float64Val(m[key]))
		ids = append(ids, id)
	}
	return ids, nil
}

// FilesFromPaths reads local files for upload.
func FilesFromPaths(paths []string) ([]FilePart, error) {
	out := make([]FilePart, 0, len(paths))
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		ct := mime.TypeByExtension(filepath.Ext(p))
		out = append(out, FilePart{Filename: filepath.Base(p), Content: b, ContentType: ct})
	}
	return out, nil
}

func float64Val(v any) float64 {
	f, _ := v.(float64)
	return f
}

