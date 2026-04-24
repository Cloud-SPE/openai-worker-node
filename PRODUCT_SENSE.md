# PRODUCT_SENSE — openai-worker-node

## What we're building

Infrastructure. The payee-side HTTP adapter that turns a host running inference servers plus a `livepeer-payment-daemon` into a sellable "worker" on the Livepeer BYOC network. Not a product for end users; a product for operators who already run inference hardware and want to monetize it through the bridge.

## Who uses this

Three personas.

### The worker operator

Someone with GPU capacity who runs inference backends (vLLM, diffusers, whisper, TTS) and wants to sell tokens / images / audio-seconds for Livepeer micropayments. They care about:

- Deployment simplicity: one config file, one binary, stands up in an afternoon.
- Income correctness: every paid request is actually paid; the ledger never credits something it didn't receive.
- Not running `go-livepeer`. The transcoding network's orchestrator machinery is not part of their operation.
- Observability: which model is busy, which backend is flaky, where are the tickets going.
- Per-model pricing they can tune per (capability, model) without code changes.

### The bridge operator

Someone running `openai-livepeer-bridge` who lists this worker in their `nodes.yaml`. They care about:

- A predictable HTTP contract (`/health`, `/capabilities`, `/quote`, paid routes).
- A truthful `/capabilities` response — what the worker advertises is what the worker can deliver.
- Fast quote refresh (batched `/quotes` endpoint).
- Clean error modes: payment-required, capacity-exceeded, model-not-loaded are distinguishable.

### The consumer-app developer (indirect)

End customers never see this worker — they talk to the bridge. But they care (indirectly) about:

- Low latency added by the payment middleware.
- No request corruption: the worker MUST pass OpenAI-compatible bodies unchanged.

## What "good" looks like

- An operator reads `DESIGN.md`, writes a `worker.yaml`, runs `docker compose up`, and starts serving paid requests within an hour.
- `openai-worker-node` adds < 5 ms of p50 latency on top of the backend for non-streaming calls, dominated by the two gRPC round-trips to the payee daemon.
- Adding a new model to an existing capability is a YAML edit and a restart. No code change.
- Adding a new capability (new OpenAI route) is one exec-plan and one new module package. The module-registration surface is small enough that the change is predominantly the estimator and the metering logic.
- The worker has never credited a request it did not first validate via `ProcessPayment`. Enforced by lint, not vigilance.

## Anti-goals

- Not a go-livepeer replacement on the transcoding network. Different game entirely.
- Not a general HTTP reverse proxy. Routes are fixed per capability module; a custom-workloads pathway exists but goes through the same registration surface, not a passthrough escape hatch.
- Not a multi-tenant service. One config, one operator, one set of backends per process.
- Not a queue. If a backend is saturated, the worker returns 503; it does not buffer requests.
- Not a DB-of-record. The payee daemon holds the ledger; this worker is a stateless adapter.

## Non-ambitions (worth naming)

- No admin UI. Observability is metrics + logs + the payee daemon's `ListPendingRedemptions` RPC.
- No automatic backend discovery. Backends are configured by URL in `worker.yaml`.
- No hot reload. Config changes restart both the worker and the daemon.
