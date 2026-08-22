# Kubernetes deployment

The Helm chart deploys the CUDAOps API, one GPU worker, Redis, and the persistent data required to share input and output files between the API and worker.

## Prerequisites

- Kubernetes cluster with the NVIDIA GPU Operator or NVIDIA device plugin installed on GPU nodes. The worker requests `nvidia.com/gpu` and selects compute capability 12.x nodes by default because the current CUDA binary targets `sm_120`.
- A `ReadWriteMany` storage class, or an existing RWX PVC. The API and worker must mount the same volume; a node-local `ReadWriteOnce` volume is not sufficient.
- Container images for the `api` and `worker` Dockerfile targets in an accessible registry. Set both image repositories and tags before installation; the chart defaults are naming conventions, not published image guarantees.
- Prometheus configured to honor standard `prometheus.io` pod annotations if application metrics should be scraped. DCGM Exporter is a cluster-level component and is not installed by this chart.

For full GPU telemetry, install NVIDIA GPU Operator on the cluster with driver management disabled when nodes already provide the NVIDIA runtime. The operator supplies the device plugin, DCGM Exporter, and `nvidia.com/gpu` resource registration. Do not use its driver installation path for the WSL environment described in this repository, because WSL projects the Windows display driver into Linux.

GPU Feature Discovery normally labels compatible nodes with `nvidia.com/gpu.compute.major=12`. Verify the label before installing:

```bash
kubectl get nodes -L nvidia.com/gpu.compute.major,nvidia.com/gpu.product
```

Clusters without GPU Feature Discovery must apply a truthful compatibility label or override `worker.nodeSelector` with an equivalent node label. The default fails closed on incompatible or unlabeled nodes instead of scheduling a worker whose CUDA binary cannot run.

## Install

Create a values file with your immutable image tags and storage class:

```yaml
api:
  image:
    repository: registry.example/cudaops-api
    tag: "<immutable-tag>"
worker:
  image:
    repository: registry.example/cudaops-worker
    tag: "<immutable-tag>"
data:
  storageClass: nfs-rwx
```

Release tags (`v*`) publish the Dockerfile's `api` and `worker` targets to GitHub Container Registry as `ghcr.io/dericsanandres/cudaops-platform-api` and `ghcr.io/dericsanandres/cudaops-platform-worker`. Use the corresponding immutable release or SHA tag in production values; private packages require `imagePullSecrets`.

Install the GPU deployment:

```bash
helm upgrade --install cudaops deploy/helm/cudaops --namespace cudaops --create-namespace --values values.yaml
kubectl -n cudaops rollout status deployment/cudaops-cudaops-api
kubectl -n cudaops rollout status deployment/cudaops-cudaops-worker
```

For a CPU-only cluster, add the supplied override. An `auto` job uses CPU fallback; an explicit `cuda` request remains an expected terminal failure.

```bash
helm upgrade --install cudaops deploy/helm/cudaops --namespace cudaops --create-namespace --values values.yaml --values deploy/helm/cudaops/values-cpu.yaml
```

Port-forward the API for the existing acceptance flow:

```bash
kubectl -n cudaops port-forward service/cudaops-cudaops-api 8080:8080
```

## Operational notes

- The chart deliberately creates one worker because the application currently processes one image at a time and shared-file storage is part of the job contract.
- Redis persistence preserves stream state across pod restarts. It has no authentication or HA configuration, matching the project's documented v0.1 limitations.
- GPU telemetry collection, Prometheus Operator resources and rule routing, SLO enforcement, and a production Redis topology remain separate milestones and cluster-level responsibilities.
