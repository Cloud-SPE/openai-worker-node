.PHONY: build test lint lint-custom doc-lint docker-build docker-build-version clean

build:
	go build -o bin/openai-worker-node ./cmd/openai-worker-node

test:
	go test -race ./...

lint:
	golangci-lint run ./...
	$(MAKE) lint-custom

# Custom lints that enforce invariants beyond what golangci-lint expresses.
# See lint/README.md. Stubs until the relevant exec-plans land.
lint-custom:
	@echo "lint-custom: no custom lints enabled yet (see lint/README.md)"

doc-lint:
	@echo "doc-lint: placeholder until doc-gardener is wired in"

# Build the container image as tztcloud/openai-worker-node:dev. Override
# tag via DOCKER_TAG=... to publish-name it.
DOCKER_TAG ?= dev
docker-build:
	docker build --build-arg VERSION=$(DOCKER_TAG) -t tztcloud/openai-worker-node:$(DOCKER_TAG) .

docker-build-version:
	$(MAKE) docker-build DOCKER_TAG=$$(git rev-parse --short HEAD)

clean:
	rm -rf bin/
