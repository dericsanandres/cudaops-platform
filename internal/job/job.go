package job

import "time"

const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
)

type Job struct {
	ID              string  `json:"id"`
	Status          string  `json:"status"`
	RequestedDevice string  `json:"requested_device"`
	UsedDevice      string  `json:"used_device,omitempty"`
	FallbackUsed    bool    `json:"fallback_used"`
	QueueMS         int64   `json:"queue_ms"`
	ProcessingMS    int64   `json:"processing_ms"`
	ResultURL       string  `json:"result_url,omitempty"`
	ErrorCode       *string `json:"error_code"`
	InputPath       string  `json:"-"`
	OutputPath      string  `json:"-"`
	CreatedAt       int64   `json:"-"`
	StartedAt       int64   `json:"-"`
	Attempts        int     `json:"-"`
}

func New(id, device, input, output string) Job {
	return Job{ID: id, Status: StatusQueued, RequestedDevice: device, InputPath: input,
		OutputPath: output, CreatedAt: time.Now().UnixMilli()}
}
