# Local acceptance checks

Run `make test`, then start the stack with `docker compose up --build`.

1. Submit a PNG and poll its status until it succeeds. Download the result and verify its media type is `image/png`.
2. Request each of `auto`, `cpu`, and `cuda`; check `used_device` and `fallback_used` in status responses.
3. Compare CPU and CUDA result bytes with `cmp`. Borders must be black and the files must be identical.
4. Stop the worker after a job enters `running`, wait at least 60 seconds, restart it, and confirm the pending stream entry is reclaimed with no more than two attempts.
5. Start with `compose.cpu.yaml`; an `auto` request must succeed on CPU while an explicit `cuda` request must fail.
6. Upload a corrupt PNG/JPEG, unsupported content, and a payload over 20 MiB; verify clean terminal errors or HTTP validation errors.
7. Inspect the CUDAOps Grafana dashboard for throughput, success/failure totals, queue latency, CPU/CUDA latency, fallbacks, and retries.

GPU correctness and performance results are deliberately local-only because GitHub-hosted CI has no compatible GPU.

