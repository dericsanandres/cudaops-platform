package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/dericsanandres/cudaops-platform/internal/config"
	"github.com/dericsanandres/cudaops-platform/internal/job"
	metricspkg "github.com/dericsanandres/cudaops-platform/internal/metrics"
	"github.com/dericsanandres/cudaops-platform/internal/processor"
	"github.com/dericsanandres/cudaops-platform/internal/store"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type worker struct {
	store       *store.Redis
	runner      processor.Runner
	metrics     *metricspkg.Metrics
	logger      *slog.Logger
	consumer    string
	timeout     time.Duration
	maxAttempts int
}

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("component", "worker")
	redisStore := store.New(cfg.RedisAddr)
	defer redisStore.Close()
	if err := waitForRedis(context.Background(), redisStore, 30*time.Second); err != nil {
		logger.Error("Redis unavailable", "error", err)
		os.Exit(1)
	}
	if err := redisStore.EnsureGroup(context.Background()); err != nil {
		logger.Error("failed to initialize stream", "error", err)
		os.Exit(1)
	}
	w := &worker{store: redisStore, runner: processor.Runner{Path: cfg.Processor},
		metrics: metricspkg.New(prometheus.DefaultRegisterer), logger: logger,
		consumer: "worker-" + uuid.NewString(), timeout: cfg.ProcessTimeout, maxAttempts: cfg.MaxAttempts}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(out http.ResponseWriter, _ *http.Request) { out.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /readyz", func(out http.ResponseWriter, request *http.Request) {
		if err := redisStore.Ping(request.Context()); err != nil {
			http.Error(out, "not ready", http.StatusServiceUnavailable)
			return
		}
		out.WriteHeader(http.StatusOK)
	})
	mux.Handle("GET /metrics", promhttp.Handler())
	server := &http.Server{Addr: ":" + cfg.WorkerPort, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		logger.Info("worker metrics listening", "address", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("metrics server stopped", "error", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	logger.Info("worker started", "consumer", w.consumer)
	if err := w.loop(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("worker stopped", "error", err)
		os.Exit(1)
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}

func waitForRedis(ctx context.Context, redisStore *store.Redis, limit time.Duration) error {
	deadline := time.Now().Add(limit)
	var err error
	for time.Now().Before(deadline) {
		if err = redisStore.Ping(ctx); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return err
}

func (w *worker) loop(ctx context.Context) error {
	reclaim := time.NewTicker(15 * time.Second)
	defer reclaim.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-reclaim.C:
			messages, err := w.store.Reclaim(ctx, w.consumer, 60*time.Second)
			if err != nil {
				w.logger.Error("reclaim failed", "error", err)
				continue
			}
			for _, message := range messages {
				w.handle(ctx, message)
			}
		default:
			messages, err := w.store.Read(ctx, w.consumer, 2*time.Second)
			if err != nil {
				w.logger.Error("stream read failed", "error", err)
				time.Sleep(time.Second)
				continue
			}
			for _, message := range messages {
				w.handle(ctx, message)
			}
		}
	}
}

func (w *worker) handle(parent context.Context, message store.Message) {
	value, err := w.store.Get(parent, message.JobID)
	if err != nil {
		w.logger.Error("job lookup failed", "job_id", message.JobID, "error", err)
		return
	}
	if value.Status == job.StatusSucceeded || value.Status == job.StatusFailed {
		_ = w.store.Ack(parent, message.StreamID)
		return
	}
	if value.Attempts >= w.maxAttempts {
		if err := w.store.Fail(parent, value.ID, "retry_limit_exceeded", 0); err != nil {
			w.logger.Error("failed to persist retry exhaustion", "job_id", value.ID, "error", err)
			return
		}
		w.metrics.Jobs.WithLabelValues(job.StatusFailed, "unknown").Inc()
		_ = w.store.Ack(parent, message.StreamID)
		return
	}

	for {
		attempt, err := w.store.Start(parent, value.ID)
		if err != nil {
			w.logger.Error("failed to mark job running", "job_id", value.ID, "error", err)
			return
		}
		started := time.Now()
		w.metrics.Inflight.Inc()
		if attempt == 1 {
			w.metrics.QueueWait.Observe(float64(max(0, started.UnixMilli()-value.CreatedAt)) / 1000)
		}
		processCtx, cancel := context.WithTimeout(parent, w.timeout)
		report, runErr := w.runner.Run(processCtx, value.InputPath, value.OutputPath, value.RequestedDevice)
		cancel()
		w.metrics.Inflight.Dec()
		elapsed := time.Since(started)
		if runErr == nil {
			if err := w.store.Succeed(parent, value.ID, report.Device, report.FallbackUsed, elapsed.Milliseconds()); err != nil {
				w.logger.Error("failed to persist success", "job_id", value.ID, "error", err)
				return
			}
			w.metrics.Jobs.WithLabelValues(job.StatusSucceeded, report.Device).Inc()
			w.metrics.Processing.WithLabelValues(report.Device).Observe(elapsed.Seconds())
			if report.FallbackUsed {
				w.metrics.Fallback.Inc()
			}
			_ = os.Remove(value.InputPath)
			_ = w.store.Ack(parent, message.StreamID)
			w.logger.Info("job succeeded", "job_id", value.ID, "device", report.Device, "fallback_used", report.FallbackUsed, "processing_ms", elapsed.Milliseconds())
			return
		}

		var processErr *processor.Error
		if errors.As(runErr, &processErr) && processErr.Retryable && attempt < w.maxAttempts {
			w.metrics.Retries.Inc()
			if err := w.store.Requeue(parent, value.ID); err != nil {
				w.logger.Error("failed to persist retry", "job_id", value.ID, "error", err)
				return
			}
			w.logger.Warn("retrying job", "job_id", value.ID, "attempt", attempt, "error", runErr)
			time.Sleep(250 * time.Millisecond)
			continue
		}
		code := "processing_failed"
		if processErr != nil {
			code = processErr.Code
		}
		if err := w.store.Fail(parent, value.ID, code, elapsed.Milliseconds()); err != nil {
			w.logger.Error("failed to persist failure", "job_id", value.ID, "error", err)
			return
		}
		device := value.RequestedDevice
		if device == "auto" {
			device = "unknown"
		}
		w.metrics.Jobs.WithLabelValues(job.StatusFailed, device).Inc()
		_ = os.Remove(value.InputPath)
		_ = os.Remove(value.OutputPath)
		_ = w.store.Ack(parent, message.StreamID)
		w.logger.Error("job failed", "job_id", value.ID, "error_code", code, "attempt", attempt, "error", runErr)
		return
	}
}

func (w *worker) String() string { return fmt.Sprintf("worker(%s)", w.consumer) }
