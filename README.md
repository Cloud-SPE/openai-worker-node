# openai-worker-node

The payee-side HTTP adapter for Livepeer BYOC payment. Sits in front of local OpenAI-compatible inference backends (vLLM, diffusers, whisper, TTS, …), validates payment via a co-located `livepeer-payment-daemon` running over a unix socket, and serves paid requests from `openai-livepeer-bridge`.

## Architecture at a glance

```
bridge ── HTTPS ─▶ openai-worker-node ── gRPC ─▶ livepeer-payment-daemon
                           │
                           └── HTTP ─▶ inference backends (vLLM / diffusers / whisper / …)
```

One worker node = two config files:

- `worker.yaml` for `openai-worker-node`
- `payment-daemon.yaml` for `livepeer-payment-daemon`

Each `(capability, model)` pair routes to its own backend URL on the
worker side. The daemon carries the same catalog without `backend_url`;
startup fails closed if the two catalogs drift.

Capabilities (v1):

- `openai:/v1/chat/completions`
- `openai:/v1/embeddings`
- `openai:/v1/images/generations`
- `openai:/v1/images/edits`
- `openai:/v1/audio/speech`
- `openai:/v1/audio/transcriptions`

Video generation, FFMPEG live transcoding, and custom workloads are backlog.

## Documentation

- [AGENTS.md](AGENTS.md) — start here if you're an agent
- [DESIGN.md](DESIGN.md) — architecture and business domains
- [PRODUCT_SENSE.md](PRODUCT_SENSE.md) — who uses this and what "good" means
- [PLANS.md](PLANS.md) — how we track work
- [docs/design-docs/](docs/design-docs/) — catalogued design decisions
- [docs/exec-plans/active/](docs/exec-plans/active/) — in-flight work

## Contracts shared with the payment daemon

- **YAML schema:** the worker parses `worker.yaml` in [`internal/config/`](internal/config/). The daemon parses a separate `payment-daemon.yaml`. Drift between the two capability catalogs is detected at runtime via the daemon-catalog cross-check.
- **gRPC API:** the `.proto` definitions in [`internal/proto/livepeer/payments/v1/`](internal/proto/livepeer/payments/v1/) are wire-compatible with the daemon's. Regenerate Go stubs with `make proto`.

## License

TBD.
