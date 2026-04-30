# openai-worker-node

The payee-side OpenAI worker adapter for the Livepeer Network Suite.
Sits in front of local OpenAI-compatible inference backends (vLLM,
diffusers, whisper, TTS, …), validates payment via a co-located
`livepeer-payment-daemon` over a unix socket, and serves paid requests
from `livepeer-openai-gateway`.

## Architecture at a glance

```
gateway ── HTTPS ─▶ openai-worker-node ── gRPC ─▶ livepeer-payment-daemon
                           │
                           └── HTTP ─▶ inference backends (vLLM / diffusers / whisper / …)
```

One worker node = one shared `worker.yaml` consumed by both
`openai-worker-node` and `livepeer-payment-daemon` (receiver mode).
Each `(capability, offering)` pair routes to its own backend URL on the
worker side; the daemon reads the same catalog and ignores
`backend_url`. Startup fails closed if the shared catalog drifts from
what the daemon advertises over `ListCapabilities`.

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

- **YAML schema:** both processes parse the same `worker.yaml`. The
  worker validates worker-facing fields and captures `payment_daemon`
  opaquely; the daemon validates its own section independently. Drift
  between the shared capability catalog and the daemon RPC surface is
  detected at runtime via the daemon-catalog cross-check.
- **gRPC API:** the `.proto` definitions in [`internal/proto/livepeer/payments/v1/`](internal/proto/livepeer/payments/v1/) are wire-compatible with the daemon's. Regenerate Go stubs with `make proto`.

## License

TBD.
