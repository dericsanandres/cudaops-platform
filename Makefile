.PHONY: build test test-go test-processor fmt vet compose-config benchmark helm-template

build:
	go build ./cmd/...
	cmake -S processor -B build/processor -G Ninja -DCUDAOPS_ENABLE_CUDA=OFF
	cmake --build build/processor

test: test-go test-processor

test-go:
	go test ./...

test-processor:
	cmake -S processor -B build/processor -G Ninja -DCUDAOPS_ENABLE_CUDA=OFF -DBUILD_TESTING=ON
	cmake --build build/processor
	ctest --test-dir build/processor --output-on-failure

fmt:
	gofmt -w cmd internal

vet:
	go vet ./...

compose-config:
	docker compose config --quiet

benchmark:
	./scripts/benchmark.sh $${IMAGE:?set IMAGE=/path/to/image.png}

helm-template:
	helm template cudaops deploy/helm/cudaops
