.PHONY: build test lint lint-custom doc-lint proto docker-build docker-build-version docker-push clean

build:
	go build -o bin/openai-worker-node ./cmd/openai-worker-node

test:
	go test -race ./...

lint:
	golangci-lint run ./...
	$(MAKE) lint-custom

# Custom lints that enforce invariants beyond what golangci-lint expresses.
# See lint/README.md.
lint-custom:
	go run ./lint/payment-middleware-check --root .

doc-lint:
	@echo "doc-lint: placeholder until doc-gardener is wired in"

# Regenerate the local proto stubs from the .proto sources under
# internal/proto/. Run after editing any .proto file. Buf v2 + the
# remote protocolbuffers/go and grpc/go plugins handle codegen — no
# local protoc/protoc-gen-go install needed.
proto:
	cd internal/proto && buf generate

# Build the container image as tztcloud/livepeer-openai-worker-node:dev.
# Override tag via DOCKER_TAG=... to publish-name it (e.g.
# `make docker-build DOCKER_TAG=v1.1.2`).
DOCKER_TAG ?= dev
DOCKER_IMAGE ?= tztcloud/livepeer-openai-worker-node

docker-build:
	docker build \
		--build-arg VERSION=$(DOCKER_TAG) \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) .

docker-push:
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)

docker-build-version:
	$(MAKE) docker-build DOCKER_TAG=$$(git rev-parse --short HEAD)

clean:
	rm -rf bin/
