# Guide for coding agents

CUDAOps is a small CUDA learning project, not an AI model service or a production product. A Go API accepts images, Redis Streams queues jobs, and a Go worker runs a C++/CUDA Sobel processor. Use AI assistance to make changes that are easy to inspect and verify; keep the implementation proportional to this portfolio project.

## Start here

- Read [README.md](README.md) for the user-facing flow, API example, and local startup.
- Read [docs/verification.md](docs/verification.md) before claiming a feature has been tested. It separates reproducible CI checks from recorded GPU and Kubernetes acceptance.
- Read the files near a requested change before editing. Follow their names and patterns, and avoid unrelated refactors.
- Use [docs/setup-wsl.md](docs/setup-wsl.md) for GPU setup and [docs/deploy-kubernetes.md](docs/deploy-kubernetes.md) for chart requirements. Do not copy those procedures into this file.

## Find the owner of a change

| Concern | Main files |
| --- | --- |
| HTTP upload, validation, status, result | `cmd/api`, `internal/api` |
| Job state, Redis stream, retries, worker loop | `cmd/worker`, `internal/job`, `internal/store` |
| Launching the processor and reading its report | `internal/processor` |
| CPU and CUDA image processing | `processor/src`, `processor/tests` |
| Environment settings | `internal/config`, `.env.example` |
| Containers and local services | `Dockerfile`, `compose.yaml`, `compose.cpu.yaml` |
| Kubernetes deployment | `deploy/helm/cudaops` |
| Metrics and operational evidence | `internal/metrics`, `monitoring`, `docs` |
| Automated checks and image publishing | `.github/workflows` |

## Preserve these contracts

- The API writes input files to shared storage and queues metadata in Redis Streams. The worker reads the same storage. Do not put image bytes in Redis.
- `POST /v1/jobs` accepts one PNG or JPEG up to 20 MiB and `device=auto|cpu|cuda`. Status and result live under `/v1/jobs/{id}`. Results are PNG. Preserve response fields and error codes unless the task explicitly changes the API.
- `auto` uses CPU when CUDA is unavailable. An explicit `cuda` request fails without CUDA; a CUDA processing error after a device is found also fails. Report the requested device, actual device, and fallback truthfully.
- CPU and CUDA Sobel output must remain byte-identical, with zero-valued borders. The processor writes one JSON report to stdout and diagnostics to stderr.
- One worker handles one image at a time. Pending entries are reclaimed after 60 seconds; attempts are capped by `CUDAOPS_MAX_ATTEMPTS` (default 2). Preserve terminal-state acknowledgement and file cleanup.
- The CUDA binary targets `sm_120`. CPU-only checks cannot establish CUDA correctness or GPU performance.

## Run locally

- The default Compose stack reserves an NVIDIA GPU for the worker. For a machine without GPU access, use the CPU override:

  ```bash
  docker compose -f compose.yaml -f compose.cpu.yaml up --build
  ```

- The API listens on `localhost:8080`; the worker exposes health and metrics on port `8081` inside Compose. Prometheus and Grafana are configured in `compose.yaml`.
- The API and worker need both Redis and the same writable data volume. If a job cannot progress, check those dependencies before changing processing code.
- Configuration comes from `CUDAOPS_*` environment variables in `internal/config`; `.env.example` lists the main settings. Keep defaults and deployment values aligned when changing them.
- This project has no authentication, cancellation, or retention policy. Do not add platform features unless the task requires them.

## Choose checks for the change

Run commands from the repository root. Use the narrowest set that covers the behavior changed.

| Change | Checks |
| --- | --- |
| Go API, worker, store, or config | `go test ./...` and `go vet ./...` |
| C++/CPU processor | CPU CMake build and CTest below |
| Compose or Dockerfile | Both Compose config checks below; use the CI container smoke flow when runtime behavior changes |
| Helm chart | `helm lint` and both template commands below |
| Documentation only | Check links and claims against the repo, then run `git diff --check` |

CPU processor checks:

```bash
cmake -S processor -B build/processor -G Ninja -DCUDAOPS_ENABLE_CUDA=OFF -DBUILD_TESTING=ON
cmake --build build/processor
ctest --test-dir build/processor --output-on-failure
```

Compose and Helm checks:

```bash
docker compose config --quiet
docker compose -f compose.yaml -f compose.cpu.yaml config --quiet

helm lint deploy/helm/cudaops
helm template cudaops deploy/helm/cudaops > /dev/null
helm template cudaops deploy/helm/cudaops --values deploy/helm/cudaops/values-cpu.yaml > /dev/null
```

`make test` combines Go and CPU processor tests. [.github/workflows/ci.yml](.github/workflows/ci.yml) also runs Go race tests, processor fallback checks, a container smoke test, and Helm validation. GPU work needs a compatible machine and separate acceptance evidence.

## Finish the work

- Add focused tests when behavior changes. Check both the success path and the relevant failure or fallback path.
- Update README or deployment docs when a user-visible contract or setup step changes. Keep benchmark and validation claims tied to recorded evidence.
- Report what changed, which checks actually ran, and what could not be verified in the current environment. Do not describe CPU-only checks as GPU validation.
