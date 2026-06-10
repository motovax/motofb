package state

import (
	"bytes"
	"fmt"
	"mime/multipart"
)

type multipartHelper struct {
	w *multipart.Writer
	b *bytes.Buffer
}

func multipartWriter(b *bytes.Buffer) *multipartHelper {
	w := multipart.NewWriter(b)
	return &multipartHelper{w: w, b: b}
}

func (m *multipartHelper) WriteField(name, value string) error {
	return m.w.WriteField(name, value)
}

func (m *multipartHelper) WriteFile(field, filename, contentType string, data []byte) error {
	h := make(map[string][]string)
	h["Content-Disposition"] = []string{fmt.Sprintf(`form-data; name="%s"; filename="%s"`, field, filename)}
	h["Content-Type"] = []string{contentType}
	part, err := m.w.CreatePart(h)
	if err != nil {
		return err
	}
	_, err = part.Write(data)
	return err
}

func (m *multipartHelper) Close() error { return m.w.Close() }

func (m *multipartHelper) FormDataContentType() string { return m.w.FormDataContentType() }