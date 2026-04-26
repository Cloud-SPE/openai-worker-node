.PHONY: build test lint lint-custom doc-lint docker-build docker-build-version clean

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

# Build the container image as tztcloud/livepeer-openai-worker-node:dev.
# Override tag via DOCKER_TAG=... to publish-name it (e.g.
# `make docker-build DOCKER_TAG=v0.8.10`).
#
# `--build-context payment-daemon=...` feeds the sibling
# livepeer-modules-project/payment-daemon module into the Dockerfile's
# named build context. Required while go.mod carries the local
# `replace` directive; goes away once the module tags a release.
DOCKER_TAG ?= dev
DOCKER_IMAGE ?= tztcloud/livepeer-openai-worker-node
# Note: do NOT name this *_PATH (e.g. PAYMENT_DAEMON_PATH) — the *_PATH
# suffix collides with common env vars (CUDA toolchains export
# LIBRARY_PATH) and an override would silently retarget the build
# context.
PAYMENT_DAEMON_CONTEXT ?= ../livepeer-modules-project/payment-daemon

docker-build:
	docker build \
		--build-arg VERSION=$(DOCKER_TAG) \
		--build-context payment-daemon=$(PAYMENT_DAEMON_CONTEXT) \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) .

docker-push:
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)

docker-build-version:
	$(MAKE) docker-build DOCKER_TAG=$$(git rev-parse --short HEAD)

clean:
	rm -rf bin/
