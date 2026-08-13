package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dericsanandres/cudaops-platform/internal/job"
	"github.com/dericsanandres/cudaops-platform/internal/store"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const MaxImageBytes int64 = 20 << 20

type JobStore interface {
	CreateAndEnqueue(context.Context, job.Job) error
	Get(context.Context, string) (job.Job, error)
	Ping(context.Context) error
}

type Server struct {
	store   JobStore
	dataDir string
	logger  *slog.Logger
}

func New(jobStore JobStore, dataDir string, logger *slog.Logger) *Server {
	return &Server{store: jobStore, dataDir: dataDir, logger: logger}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", s.ready)
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("POST /v1/jobs", s.create)
	mux.HandleFunc("GET /v1/jobs/{id}", s.get)
	mux.HandleFunc("GET /v1/jobs/{id}/result", s.result)
	return mux
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "redis_unavailable")
		return
	}
	if err := os.MkdirAll(s.dataDir, 0o755); err != nil {
		writeError(w, http.StatusServiceUnavailable, "storage_unavailable")
		return
	}
	probe, err := os.CreateTemp(s.dataDir, ".ready-")
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "storage_unavailable")
		return
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxImageBytes+(1<<20))
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_multipart")
		return
	}
	device := "auto"
	var image []byte
	for {
		part, nextErr := reader.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			writeError(w, http.StatusBadRequest, "invalid_multipart")
			return
		}
		switch part.FormName() {
		case "device":
			value, readErr := io.ReadAll(io.LimitReader(part, 16))
			if readErr != nil {
				writeError(w, http.StatusBadRequest, "invalid_device")
				return
			}
			device = string(value)
		case "image":
			if image != nil {
				writeError(w, http.StatusBadRequest, "multiple_images")
				return
			}
			image, err = readPart(part)
			if err != nil {
				if errors.Is(err, errTooLarge) {
					writeError(w, http.StatusRequestEntityTooLarge, "image_too_large")
				} else {
					writeError(w, http.StatusBadRequest, "invalid_image")
				}
				return
			}
		}
	}
	if device != "auto" && device != "cpu" && device != "cuda" {
		writeError(w, http.StatusBadRequest, "invalid_device")
		return
	}
	ext := imageExtension(image)
	if ext == "" {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_image")
		return
	}
	if err := os.MkdirAll(s.dataDir, 0o755); err != nil {
		writeError(w, http.StatusServiceUnavailable, "storage_unavailable")
		return
	}
	id := uuid.NewString()
	input := filepath.Join(s.dataDir, id+"-input"+ext)
	output := filepath.Join(s.dataDir, id+"-output.png")
	if err := os.WriteFile(input, image, 0o600); err != nil {
		writeError(w, http.StatusServiceUnavailable, "storage_unavailable")
		return
	}
	value := job.New(id, device, input, output)
	if err := s.store.CreateAndEnqueue(r.Context(), value); err != nil {
		_ = os.Remove(input)
		s.logger.Error("enqueue failed", "job_id", id, "error", err)
		writeError(w, http.StatusServiceUnavailable, "queue_unavailable")
		return
	}
	s.logger.Info("job queued", "job_id", id, "requested_device", device)
	writeJSON(w, http.StatusAccepted, map[string]string{"id": id, "status": job.StatusQueued, "status_url": "/v1/jobs/" + id})
}

var errTooLarge = errors.New("image too large")

func readPart(part *multipart.Part) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(part, MaxImageBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > MaxImageBytes {
		return nil, errTooLarge
	}
	if len(data) == 0 {
		return nil, errors.New("empty image")
	}
	return data, nil
}

func imageExtension(data []byte) string {
	if len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n" {
		return ".png"
	}
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return ".jpg"
	}
	return ""
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	value, err := s.store.Get(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "job_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "redis_unavailable")
		return
	}
	if value.Status == job.StatusSucceeded {
		value.ResultURL = "/v1/jobs/" + value.ID + "/result"
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) result(w http.ResponseWriter, r *http.Request) {
	value, err := s.store.Get(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "job_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "redis_unavailable")
		return
	}
	if value.Status != job.StatusSucceeded {
		writeError(w, http.StatusConflict, "job_not_complete")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", value.ID+".png"))
	http.ServeFile(w, r, value.OutputPath)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSONStatus(w, status, map[string]string{"error_code": code})
}
func writeJSON(w http.ResponseWriter, status int, value any) { writeJSONStatus(w, status, value) }
func writeJSONStatus(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func IsImageName(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".png" || ext == ".jpg" || ext == ".jpeg"
}
