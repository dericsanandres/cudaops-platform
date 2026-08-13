# syntax=docker/dockerfile:1.7
FROM golang:1.26.5-bookworm AS go-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/cudaops-api ./cmd/api && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/cudaops-worker ./cmd/worker

FROM nvidia/cuda:13.1.2-devel-ubuntu24.04 AS processor-builder
RUN apt-get update && apt-get install -y --no-install-recommends cmake ninja-build g++ && rm -rf /var/lib/apt/lists/*
WORKDIR /src/processor
COPY processor ./
RUN cmake -S . -B /build -G Ninja -DCMAKE_BUILD_TYPE=Release -DCUDAOPS_ENABLE_CUDA=ON && \
    cmake --build /build

FROM ubuntu:24.04 AS api
RUN useradd --system --uid 10001 --no-create-home cudaops
COPY --from=go-builder /out/cudaops-api /usr/local/bin/cudaops-api
USER 10001
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/cudaops-api"]

FROM nvidia/cuda:13.1.2-runtime-ubuntu24.04 AS worker
RUN useradd --system --uid 10001 --no-create-home cudaops
COPY --from=go-builder /out/cudaops-worker /usr/local/bin/cudaops-worker
COPY --from=processor-builder /build/cudaops-process /usr/local/bin/cudaops-process
USER 10001
EXPOSE 8081
ENTRYPOINT ["/usr/local/bin/cudaops-worker"]

