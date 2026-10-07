package httpapi_test

import (
	"bytes"
	"mime"
	"mime/multipart"
	"net/textproto"
	"testing"
)

func multipartBody(t *testing.T, fields [][2]string, filename, contentType string, fileContent []byte) ([]byte, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, field := range fields {
		if err := writer.WriteField(field[0], field[1]); err != nil {
			t.Fatalf("WriteField: %v", err)
		}
	}
	if filename != "" {
		header := textproto.MIMEHeader{}
		header.Set("Content-Disposition",
			mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": filename}))
		header.Set("Content-Type", contentType)
		part, err := writer.CreatePart(header)
		if err != nil {
			t.Fatalf("CreatePart: %v", err)
		}
		if _, err := part.Write(fileContent); err != nil {
			t.Fatalf("write file part: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return body.Bytes(), writer.FormDataContentType()
}
