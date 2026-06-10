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

// UploadFileResult is full metadata for one uploaded file.
type UploadFileResult struct {
	ID       int64
	FileType string
	Filename string
}

// UploadFiles uploads attachments to Mercury and returns file ids.
func (s *State) UploadFiles(ctx context.Context, files []FilePart, voiceClip bool) ([]int64, error) {
	results, err := s.UploadFilesDetailed(ctx, files, voiceClip)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(results))
	for i, r := range results {
		ids[i] = r.ID
	}
	return ids, nil
}

// UploadFilesDetailed uploads attachments and returns full file metadata.
func (s *State) UploadFilesDetailed(ctx context.Context, files []FilePart, voiceClip bool) ([]UploadFileResult, error) {
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
	results := make([]UploadFileResult, 0, len(files))
	for _, v := range meta {
		m, _ := v.(map[string]any)
		ft, _ := m["filetype"].(string)
		key := internal.MIMEToKey(ft)
		id := int64(float64Val(m[key]))
		results = append(results, UploadFileResult{
			ID:       id,
			FileType: ft,
			Filename: strVal(m["filename"]),
		})
	}
	if len(results) != len(files) {
		return nil, fberr.New("UploadFilesDetailed", "some files could not be uploaded")
	}
	return results, nil
}

func strVal(v any) string {
	switch x := v.(type) {
	case string:
		return x
	default:
		if v == nil {
			return ""
		}
		return fmt.Sprint(v)
	}
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

