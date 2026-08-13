package api

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dericsanandres/cudaops-platform/internal/job"
	"github.com/dericsanandres/cudaops-platform/internal/store"
)

type fakeStore struct {
	value     job.Job
	createErr error
	getErr    error
}

func (f *fakeStore) CreateAndEnqueue(_ context.Context, value job.Job) error {
	f.value = value
	return f.createErr
}
func (f *fakeStore) Get(_ context.Context, _ string) (job.Job, error) { return f.value, f.getErr }
func (f *fakeStore) Ping(context.Context) error                       { return nil }

func TestCreateAndResultLifecycle(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeStore{}
	handler := New(fake, dir, slog.New(slog.NewTextHandler(os.Stderr, nil))).Handler()
	response := upload(t, handler, tinyPNG(), "auto")
	if response.Code != http.StatusAccepted {
		t.Fatalf("create status = %d, body=%s", response.Code, response.Body.String())
	}
	if fake.value.Status != job.StatusQueued || fake.value.RequestedDevice != "auto" {
		t.Fatalf("unexpected job: %+v", fake.value)
	}
	if filepath.Ext(fake.value.InputPath) != ".png" {
		t.Fatalf("unsafe or wrong input path: %s", fake.value.InputPath)
	}
	if fake.value.InputPath == fake.value.OutputPath {
		t.Fatal("input and output paths must remain distinct for PNG uploads")
	}

	fake.value.Status = job.StatusRunning
	request := httptest.NewRequest(http.MethodGet, "/v1/jobs/id/result", nil)
	request.SetPathValue("id", "id")
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, request)
	if result.Code != http.StatusConflict {
		t.Fatalf("unfinished result status = %d", result.Code)
	}

	fake.getErr = store.ErrNotFound
	result = httptest.NewRecorder()
	handler.ServeHTTP(result, request)
	if result.Code != http.StatusNotFound {
		t.Fatalf("unknown result status = %d", result.Code)
	}
}

func TestValidationAndEnqueueCleanup(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeStore{createErr: errors.New("redis down")}
	handler := New(fake, dir, slog.Default()).Handler()
	response := upload(t, handler, tinyPNG(), "cpu")
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", response.Code)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("orphaned files: %v", entries)
	}

	response = upload(t, handler, []byte("not an image"), "cpu")
	if response.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("invalid image status = %d", response.Code)
	}
	response = upload(t, handler, tinyPNG(), "tpu")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid device status = %d", response.Code)
	}
}

func TestImageLimit(t *testing.T) {
	handler := New(&fakeStore{}, t.TempDir(), slog.Default()).Handler()
	data := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{1}, int(MaxImageBytes))...)
	response := upload(t, handler, data, "cpu")
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
}

func upload(t *testing.T, handler http.Handler, image []byte, device string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("image", "ignored.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(image)
	_ = writer.WriteField("device", device)
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, "/v1/jobs", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func tinyPNG() []byte { return append([]byte("\x89PNG\r\n\x1a\n"), []byte(strings.Repeat("x", 16))...) }
