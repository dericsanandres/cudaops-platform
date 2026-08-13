package processor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestLaunchFailureIsRetryable(t *testing.T) {
	_, err := (Runner{Path: filepath.Join(t.TempDir(), "missing-processor")}).Run(context.Background(), "in", "out", "cpu")
	var processErr *Error
	if !errors.As(err, &processErr) || !processErr.Retryable || processErr.Code != "processor_launch_failed" {
		t.Fatalf("error = %#v", err)
	}
}

func TestTimeoutIsNotRetryable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper is only used on Unix CI")
	}
	script := filepath.Join(t.TempDir(), "slow-processor")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 5\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := (Runner{Path: script}).Run(ctx, "in", "out", "cpu")
	var processErr *Error
	if !errors.As(err, &processErr) || processErr.Retryable || processErr.Code != "processing_timeout" {
		t.Fatalf("error = %#v", err)
	}
}

func TestInvalidReportFailsWithoutRetry(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper is only used on Unix CI")
	}
	script := filepath.Join(t.TempDir(), "bad-processor")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho not-json\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := (Runner{Path: script}).Run(context.Background(), "in", "out", "cpu")
	var processErr *Error
	if !errors.As(err, &processErr) || processErr.Retryable || processErr.Code != "invalid_processor_report" {
		t.Fatalf("error = %#v", err)
	}
}
