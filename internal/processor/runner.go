package processor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type Report struct {
	Device       string  `json:"device"`
	FallbackUsed bool    `json:"fallback_used"`
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	TotalMS      float64 `json:"total_ms"`
	KernelMS     float64 `json:"kernel_ms"`
}

type Error struct {
	Code      string
	Retryable bool
	Cause     error
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %v", e.Code, e.Cause) }
func (e *Error) Unwrap() error { return e.Cause }

type Runner struct{ Path string }

func (r Runner) Run(ctx context.Context, input, output, device string) (Report, error) {
	command := exec.CommandContext(ctx, r.Path, "--input", input, "--output", output, "--device", device)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return Report{}, &Error{Code: "processing_timeout", Cause: ctx.Err()}
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			code := "processing_failed"
			message := strings.ToLower(stderr.String())
			if strings.Contains(message, "decode failed") {
				code = "invalid_image"
			}
			if strings.Contains(message, "out of memory") {
				code = "cuda_oom"
			}
			return Report{}, &Error{Code: code, Cause: fmt.Errorf("%s", strings.TrimSpace(stderr.String()))}
		}
		return Report{}, &Error{Code: "processor_launch_failed", Retryable: true, Cause: err}
	}
	var report Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		return Report{}, &Error{Code: "invalid_processor_report", Cause: err}
	}
	return report, nil
}
