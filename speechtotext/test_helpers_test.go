package speechtotext

import (
	"io"
	"mime/multipart"
	"net/http"
	"testing"
)

func readMultipartForm(t *testing.T, r *http.Request) *multipartForm {
	t.Helper()

	mr, err := r.MultipartReader()
	if err != nil {
		t.Fatalf("multipart reader: %v", err)
	}
	form, err := mr.ReadForm(1024 * 1024)
	if err != nil {
		t.Fatalf("read form: %v", err)
	}
	t.Cleanup(func() {
		_ = form.RemoveAll()
	})

	return &multipartForm{
		Value: form.Value,
		File:  form.File,
	}
}

func readMultipartFile(t *testing.T, r *http.Request) (*multipartForm, string) {
	t.Helper()

	form := readMultipartForm(t, r)
	files := form.File["file"]
	if len(files) != 1 {
		t.Fatalf("file parts = %d, want 1", len(files))
	}

	file, err := files[0].Open()
	if err != nil {
		t.Fatalf("open uploaded file: %v", err)
	}
	defer file.Close()

	body, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("read uploaded file: %v", err)
	}

	return form, string(body)
}

type multipartForm struct {
	Value map[string][]string
	File  map[string][]*multipart.FileHeader
}
