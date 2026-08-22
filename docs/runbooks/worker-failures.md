# Worker failure runbook

## Signals

- `CUDAOpsJobFailure`: a job reached a terminal failed state.
- `CUDAOpsWorkerRetry`: the worker retried a processor launch.
- `CUDAOpsCUDAFallback`: an `auto` job ran on CPU after CUDA was unavailable.

## Triage

1. Identify the affected job ID from API and worker logs.
2. Inspect its status at `GET /v1/jobs/{id}` and record `error_code`, `attempts`, `requested_device`, `used_device`, and `fallback_used`.
3. Check the worker pod readiness, restart count, and GPU allocation with `kubectl -n <namespace> describe pod <worker-pod>`.
4. Confirm the node reports `nvidia.com/gpu` and inspect GPU Operator and DCGM Exporter pods before changing application configuration.
5. For `processor_launch_failed`, inspect worker logs and preserve the input artifact if available. The worker makes at most the configured number of attempts.

## Recovery

- A transient worker interruption is reclaimed after 60 seconds by design. Do not manually requeue it before that period elapses.
- For automatic fallbacks, confirm whether CPU completion is acceptable for the workload. Explicit `cuda` requests intentionally fail instead of falling back.
- If a node-level GPU issue persists, cordon the node or scale the worker to zero while the cluster GPU operator is remediated. Restore the worker only after `nvidia.com/gpu` capacity returns.

## Evidence to retain

Keep the job status payload, relevant worker logs, node description, the alert timestamp, and the Prometheus query result. Do not record uploaded image contents in incident notes.
