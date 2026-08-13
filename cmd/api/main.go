package main

import (
	"log/slog"
	"net/http"
	"os"

	apihttp "github.com/dericsanandres/cudaops-platform/internal/api"
	"github.com/dericsanandres/cudaops-platform/internal/config"
	"github.com/dericsanandres/cudaops-platform/internal/store"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("component", "api")
	redisStore := store.New(cfg.RedisAddr)
	defer redisStore.Close()
	server := &http.Server{Addr: ":" + cfg.APIPort, Handler: apihttp.New(redisStore, cfg.DataDir, logger).Handler(), ReadHeaderTimeout: 5_000_000_000}
	logger.Info("API listening", "address", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("API stopped", "error", err)
		os.Exit(1)
	}
}
