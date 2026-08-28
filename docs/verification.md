# Verification evidence

This page separates publicly reproducible checks from acceptance recorded on the local GPU and CPU-only Kubernetes environments. It is evidence for a learning project, not a production certification or a general performance claim.

## Publicly reproducible checks

The [`CI` workflow](../.github/workflows/ci.yml) and [`Security` workflow](../.github/workflows/security.yml) run on pull requests and `main`. The v0.2.0 baseline passed in [CI run 33145436808](https://github.com/dericsanandres/cudaops-platform/actions/runs/33145436808) and [CodeQL run 33145436849](https://github.com/dericsanandres/cudaops-platform/actions/runs/33145436849).

| Area | Check performed |
|---|---|
| Go | Formatting, `go vet ./...`, and `go test -race ./...` |
| CPU processor | CMake/Ninja build and CTest |
| Processor behavior | CPU execution, automatic CPU fallback, byte parity between those paths, explicit CUDA failure without a GPU, corrupt PNG rejection, and invalid-device rejection |
| Compose and containers | Default and CPU-only Compose validation, worker image build, and an end-to-end CPU fallback smoke test with a valid PNG result |
| Helm | Chart lint plus default GPU and CPU-only template rendering |
| Security | CodeQL analysis of Go in autobuild mode |

Run the narrow local equivalents from the repository root:

```bash
make test
make vet
docker compose config --quiet
docker compose -f compose.yaml -f compose.cpu.yaml config --quiet
helm lint deploy/helm/cudaops
helm template cudaops deploy/helm/cudaops > /dev/null
helm template cudaops deploy/helm/cudaops \
  --values deploy/helm/cudaops/values-cpu.yaml > /dev/null
```

The container smoke test itself is preserved in the [CI workflow](../.github/workflows/ci.yml), including the generated test PNG, API polling, fallback assertions, result download, and PNG signature check.

## Recorded local GPU acceptance

The full local stack was accepted with this environment:

| Component | Accepted value |
|---|---|
| Laptop | ASUS TUF Gaming A16 FA608UM |
| GPU | NVIDIA GeForce RTX 5060 Laptop GPU |
| VRAM | 8 GB |
| Compute capability | 12.0 / `sm_120` |
| CUDA Toolkit | 13.1 |
| Linux environment | Ubuntu 24.04 under WSL2 |

The recorded checks passed for:

- PNG and JPEG upload, processing, status polling, and PNG result retrieval.
- `auto`, `cpu`, and `cuda` device requests with truthful `used_device` and `fallback_used` fields.
- Byte-identical CPU and CUDA results, including black output borders.
- Recovery after interrupting the worker during a running job and restarting it after the reclaim interval.
- Retry behavior capped at the configured two attempts.
- Clean errors for corrupt PNG/JPEG input, unsupported content, oversized payloads, invalid device requests, and explicit CUDA requests in CPU-only mode.
- Prometheus scraping of the API and worker metrics.
- The provisioned Grafana dashboard showing throughput, success and failure totals, queue latency, CPU/CUDA processing latency, fallbacks, and retries.

GPU correctness remains local-only because GitHub-hosted CI has no compatible GPU.

### Reproduce the acceptance flow

Start the GPU stack with `docker compose up --build`, then:

1. Submit both a PNG and JPEG and poll each status until it succeeds. Download each result and verify its media type and PNG signature.
2. Request each of `auto`, `cpu`, and `cuda`; compare `requested_device`, `used_device`, and `fallback_used` in the status responses.
3. Process the same source through CPU and CUDA and compare the result bytes with `cmp`. Verify that output borders are black.
4. Stop the worker after a job enters `running`, wait at least 60 seconds, restart it, and confirm that the pending Redis stream entry is reclaimed with no more than two attempts.
5. Start with `compose.cpu.yaml`; confirm that `auto` succeeds on CPU and an explicit `cuda` request reaches a clean terminal failure.
6. Upload corrupt PNG/JPEG data, unsupported content, and a payload over 20 MiB; verify the expected HTTP validation or terminal job errors.
7. Inspect Prometheus targets and the CUDAOps Grafana dashboard for the recorded application signals.

## Honest benchmark record

The retained benchmark used one 4096×4096 input, one warm-up, and 20 measured runs per device. [`scripts/benchmark.sh`](../scripts/benchmark.sh) reports the median and nearest-rank p95 from the processor's `total_ms` value.

| Device | Median | p95 |
|---|---:|---:|
| CPU | 6715.079 ms | 7613.211 ms |
| CUDA | 5328.524 ms | 8123.004 ms |

The CPU median divided by the CUDA median is `1.26×`. The CUDA p95 was slower in this sample. These values cover the complete command-line processor path—image decode, Sobel processing, and PNG encode—and do not isolate kernel execution. They describe this workload, implementation, and laptop only.

To repeat the same measurement with a suitable 4096×4096 image and GPU-capable processor build:

```bash
IMAGE=/path/to/image.png make benchmark
```

The script defaults to 20 measured runs and allows a different count as its second argument.

## Published artifacts

The tag-triggered [Publish Images run 32614290014](https://github.com/dericsanandres/cudaops-platform/actions/runs/32614290014) completed successfully for both Dockerfile targets. Public manifest resolution and pulls completed for:

| Image | v0.2.0 digest |
|---|---|
| `ghcr.io/dericsanandres/cudaops-platform-api:0.2.0` | `sha256:72bda398f3359435e4ebf9b166110ea8a40427ca0ede943aa81a92d4450aa5c3` |
| `ghcr.io/dericsanandres/cudaops-platform-worker:0.2.0` | `sha256:10c42ed12001fe5df4731d9a085b4d27bf762c5184d9afc77dbff2fd17cb9fbe` |

The digests identify the exact single-platform manifests accepted after release. They can be resolved without repository credentials:

```bash
docker buildx imagetools inspect ghcr.io/dericsanandres/cudaops-platform-api:0.2.0
docker buildx imagetools inspect ghcr.io/dericsanandres/cudaops-platform-worker:0.2.0
```

## Recorded Kubernetes acceptance

Post-release CPU-only acceptance passed on a kind cluster running Kubernetes v1.37.0 and using the exact v0.2.0 API and worker images above.

- Both the Redis data claim and shared job-data claim reached `Bound`.
- The API, worker, and Redis workloads reached Ready with the expected release images.
- An in-cluster request reached the API Service, queued a job, and completed through the worker.
- An `auto` device request reported CPU use with `fallback_used: true`.
- The downloaded result had a valid PNG signature.
- The test release and kind cluster were removed cleanly after acceptance.

This confirms the CPU-only chart path, public image pulls, shared storage, service routing, queue, processor, and result flow. It does not validate GPU scheduling or GPU execution in Kubernetes.

## Verification boundaries

The following remain untested:

- Scheduling and running the worker on a Kubernetes GPU node with the compute-major-12 label and NVIDIA device resource.
- Prometheus Operator discovery of the optional ServiceMonitors, evaluation of the PrometheusRules, and downstream alert routing.

The chart also does not claim production Redis, storage, authentication, retention, SLO enforcement, or cluster-managed GPU telemetry.
