# openai-worker-node

The payee-side HTTP adapter for Livepeer BYOC payment. Sits in front of local OpenAI-compatible inference backends (vLLM, diffusers, whisper, TTS, …), validates payment via a co-located [`livepeer-payment-daemon`](../livepeer-modules-project/payment-daemon), and serves paid requests from [`openai-livepeer-bridge`](../openai-livepeer-bridge).

## Status

Scaffolding. Tracked under [`docs/exec-plans/active/0001-repo-scaffold.md`](docs/exec-plans/active/0001-repo-scaffold.md). No source code yet.

## Architecture at a glance

```
bridge ── HTTPS ─▶ openai-worker-node ── gRPC ─▶ livepeer-payment-daemon
                           │
                           └── HTTP ─▶ inference backends (vLLM / diffusers / whisper / …)
```

One worker node = one config file (`worker.yaml`), bind-mounted into both the worker process and the payment daemon. Each (capability, model) pair routes to its own backend URL.

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

## Cross-repo contracts

- **YAML schema:** [`livepeer-modules-project/payment-daemon/docs/design-docs/shared-yaml.md`](../livepeer-modules-project/payment-daemon/docs/design-docs/shared-yaml.md)
- **gRPC API:** [`livepeer-modules-project/payment-daemon/proto/livepeer/payments/v1/payee_daemon.proto`](../livepeer-modules-project/payment-daemon/proto/livepeer/payments/v1/payee_daemon.proto)

## License

TBD.
