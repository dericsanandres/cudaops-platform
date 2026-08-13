package store

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/dericsanandres/cudaops-platform/internal/job"
	"github.com/redis/go-redis/v9"
)

const (
	stream = "cudaops:jobs"
	group  = "cudaops-workers"
)

var ErrNotFound = errors.New("job not found")

type Redis struct{ client *redis.Client }

func New(addr string) *Redis                    { return &Redis{client: redis.NewClient(&redis.Options{Addr: addr})} }
func (r *Redis) Close() error                   { return r.client.Close() }
func (r *Redis) Ping(ctx context.Context) error { return r.client.Ping(ctx).Err() }

func key(id string) string { return "cudaops:job:" + id }

func (r *Redis) CreateAndEnqueue(ctx context.Context, value job.Job) error {
	_, err := r.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(ctx, key(value.ID), map[string]any{
			"id": value.ID, "status": value.Status, "requested_device": value.RequestedDevice,
			"input_path": value.InputPath, "output_path": value.OutputPath,
			"created_at": value.CreatedAt, "attempts": 0,
		})
		pipe.XAdd(ctx, &redis.XAddArgs{Stream: stream, Values: map[string]any{"id": value.ID}})
		return nil
	})
	return err
}

func (r *Redis) Get(ctx context.Context, id string) (job.Job, error) {
	values, err := r.client.HGetAll(ctx, key(id)).Result()
	if err != nil {
		return job.Job{}, err
	}
	if len(values) == 0 {
		return job.Job{}, ErrNotFound
	}
	created, _ := strconv.ParseInt(values["created_at"], 10, 64)
	started, _ := strconv.ParseInt(values["started_at"], 10, 64)
	attempts, _ := strconv.Atoi(values["attempts"])
	queueMS, _ := strconv.ParseInt(values["queue_ms"], 10, 64)
	processingMS, _ := strconv.ParseInt(values["processing_ms"], 10, 64)
	fallback, _ := strconv.ParseBool(values["fallback_used"])
	var errorCode *string
	if value := values["error_code"]; value != "" {
		errorCode = &value
	}
	return job.Job{ID: id, Status: values["status"], RequestedDevice: values["requested_device"],
		UsedDevice: values["used_device"], FallbackUsed: fallback, QueueMS: queueMS,
		ProcessingMS: processingMS, ErrorCode: errorCode, InputPath: values["input_path"],
		OutputPath: values["output_path"], CreatedAt: created, StartedAt: started, Attempts: attempts}, nil
}

func (r *Redis) EnsureGroup(ctx context.Context) error {
	err := r.client.XGroupCreateMkStream(ctx, stream, group, "0").Err()
	if err != nil && !errors.Is(err, redis.Nil) && !isBusyGroup(err) {
		return err
	}
	return nil
}

func isBusyGroup(err error) bool {
	return err != nil && len(err.Error()) >= 9 && err.Error()[:9] == "BUSYGROUP"
}

type Message struct{ StreamID, JobID string }

func (r *Redis) Read(ctx context.Context, consumer string, block time.Duration) ([]Message, error) {
	streams, err := r.client.XReadGroup(ctx, &redis.XReadGroupArgs{Group: group, Consumer: consumer,
		Streams: []string{stream, ">"}, Count: 1, Block: block}).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return messages(streams), nil
}

func (r *Redis) Reclaim(ctx context.Context, consumer string, idle time.Duration) ([]Message, error) {
	claimed, _, err := r.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{Stream: stream, Group: group,
		Consumer: consumer, MinIdle: idle, Start: "0-0", Count: 10}).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return convertMessages(claimed), nil
}

func convertMessages(items []redis.XMessage) []Message {
	result := make([]Message, 0, len(items))
	for _, item := range items {
		if id, ok := item.Values["id"].(string); ok {
			result = append(result, Message{item.ID, id})
		}
	}
	return result
}

func messages(streams []redis.XStream) []Message {
	var result []Message
	for _, value := range streams {
		result = append(result, convertMessages(value.Messages)...)
	}
	return result
}

func (r *Redis) Start(ctx context.Context, id string) (int, error) {
	now := time.Now().UnixMilli()
	attempts, err := r.client.HIncrBy(ctx, key(id), "attempts", 1).Result()
	if err != nil {
		return 0, err
	}
	created, _ := r.client.HGet(ctx, key(id), "created_at").Int64()
	err = r.client.HSet(ctx, key(id), map[string]any{"status": job.StatusRunning, "started_at": now,
		"queue_ms": max(0, now-created)}).Err()
	return int(attempts), err
}

func (r *Redis) Succeed(ctx context.Context, id, device string, fallback bool, elapsedMS int64) error {
	return r.client.HSet(ctx, key(id), map[string]any{"status": job.StatusSucceeded,
		"used_device": device, "fallback_used": fallback, "processing_ms": elapsedMS, "error_code": ""}).Err()
}

func (r *Redis) Fail(ctx context.Context, id, code string, elapsedMS int64) error {
	return r.client.HSet(ctx, key(id), map[string]any{"status": job.StatusFailed,
		"processing_ms": elapsedMS, "error_code": code}).Err()
}

func (r *Redis) Requeue(ctx context.Context, id string) error {
	return r.client.HSet(ctx, key(id), "status", job.StatusQueued).Err()
}

func (r *Redis) Ack(ctx context.Context, streamID string) error {
	return r.client.XAck(ctx, stream, group, streamID).Err()
}
