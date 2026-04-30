# Running with Docker

The `compose.yaml` at the repo root stands up `openai-worker-node`
alongside `livepeer-payment-daemon` (receiver mode), with one
shared `worker.yaml` and one shared unix socket.

## Prerequisites

- Docker 24+ with Compose v2.

The dev `compose.yaml` pulls the `payment-daemon` sidecar as a published image (`tztcloud/livepeer-payment-daemon`); the worker image builds from this repo alone. No sibling checkout required.

## First run (dev mode, fake broker)

1. Copy the annotated example configs:

   ```bash
   cp worker.example.yaml worker.yaml
   ```

2. Edit `worker.yaml` for fake-broker dev mode:
   - Under `payment_daemon.broker`, set `mode: fake`.
   - Drop `rpc_url` and `ticket_broker_contract`.
   - Add `fake_sender_balances_wei` with the
     bridge's ETH address and a generous balance, e.g.:

     ```yaml
     fake_sender_balances_wei:
       "0xBRIDGE_ADDR_HERE": "1000000000000000000"
     ```

   - Replace `payment_daemon.recipient_eth_address` with a real-looking address
     (the format check requires 0x + 40 hex chars, but the value
     doesn't need to be a real wallet in fake mode).
   - Set each `capabilities[].offerings[].backend_url` to a reachable
     inference backend.

3. Bring the stack up:

   ```bash
   docker compose up --build
   ```

   The first build takes 1–2 minutes (module download + two Go
   builds). Subsequent builds are fast thanks to layer caching.

4. Verify:

   ```bash
   curl -s http://localhost:8080/health | jq
   # {"status":"ok","api_version":1,"protocol_version":1,"max_concurrent":16}

   curl -s http://localhost:8080/registry/offerings | jq
   # {"capabilities":[...]}
   ```

## Production mode (real broker)

1. Keep `payment_daemon.broker.mode: ethereum` in `worker.yaml`.
2. Point `payment_daemon.broker.rpc_url` at your JSON-RPC endpoint and set
   `ticket_broker_contract` to the `TicketBroker` contract address for
   the chain you're deploying to.
3. Mount a real V3 JSON keystore into the `payment-daemon` container
   and set the daemon CLI / compose wiring to match:
   `--keystore-path=/etc/livepeer/keystore.json`,
   `--keystore-password-file=/etc/livepeer/keystore-password`, and
   `--store-path=/var/lib/livepeer/payment-daemon.db`.
4. Make sure every `capabilities[].offerings[].backend_url` in
   `worker.yaml` points at an actual inference server the worker can
   reach.
5. Keep the shared capability catalog byte-identical between what the
   worker parses locally and what the daemon returns via
   `ListCapabilities`.

## How the shared config works

The same `worker.yaml` is bind-mounted into both containers at
`/etc/livepeer/worker.yaml`.

The worker's startup cross-checks further by calling
`PayeeDaemon.ListCapabilities` over the unix socket and asserting
byte-equality with its own parse for every
`(capability, offering, price)`
triple. Flipping `verify_daemon_consistency_on_start: false` disables
this check — only do so in dev where you know you're out of lockstep.

## Log level

`command: [--log-level=info]` in `compose.yaml` is the default. Valid
values: `error`, `warn`, `info`, `debug`. Debug logs include per-
request payment details (sender, work_id, work_units) that are too
chatty for production but invaluable during integration.

## Observability

The worker can expose Prometheus metrics on a separate listener. Off
by default — enable by setting `WORKER_METRICS_PORT` in `.env` (or
passing `--metrics-listen=:9093` directly when running the binary outside
Docker).

Two flags control the listener:

| Flag | Default | Notes |
|---|---|---|
| `--metrics-listen` | `""` (off) | `host:port`. Empty = no listener. Worker port allocation is `:9093`. |
| `--metrics-max-series-per-metric` | `10000` | Hard cap on distinct label tuples per metric. `0` disables the cap. New tuples beyond the cap are dropped (a `WARN` is logged once per metric). |

When enabled, the listener serves:

- `GET /metrics` — Prometheus exposition (the metrics catalog is in [`docs/design-docs/metrics.md`](../design-docs/metrics.md)).
- `GET /healthz` — plain `ok\n` for the metrics process. The main worker `/health` lives on the regular HTTP port.
- `GET /` — index page listing the endpoints above.

### Sample Prometheus scrape config

```yaml
scrape_configs:
  - job_name: livepeer-openai-worker
    scrape_interval: 30s
    static_configs:
      - targets: ['worker.example:9093']
        labels:
          worker: 'vps-1'
```

For a multi-worker deployment, expand `static_configs` (or use
`file_sd_configs` for dynamic discovery). The `worker` label is
optional but useful — it survives in Grafana so you can filter
dashboards by VPS.

### Production exposure

`compose.prod.yaml` does NOT publish the metrics port to the host by
default — the line is commented out. Uncomment after setting
`WORKER_METRICS_PORT`:

```yaml
ports:
  - '127.0.0.1:${WORKER_METRICS_PORT}:${WORKER_METRICS_PORT}'
```

Bind to `127.0.0.1` if your Prometheus runs on the same VPS;
drop the prefix only if you've firewalled the port at the network
level. The endpoint is unauthenticated — `/metrics` enumerates every
configured `(capability, model)` pair, which is operationally
sensitive.

## Upgrading

To pick up a new daemon or worker release, bump `PAYMENT_IMAGE_TAG` and
`WORKER_IMAGE_TAG` in `.env` (or rely on the default `v3.0.1` style pins
in compose) and run:

```
docker compose pull && docker compose up -d
```

Schema or proto changes from the daemon side land here as PRs that
update `internal/config/` (worker.yaml schema) and/or
`internal/proto/livepeer/payments/v1/` (regenerate with `make proto`).
Drift between this repo's pinned proto/schema and a running daemon is
caught at startup by `VerifyDaemonCatalog` — the worker refuses to
start on mismatch rather than serving wrong-priced requests.

## CI image publishing

GitHub Actions publishes `tztcloud/livepeer-openai-worker-node` on tag
pushes matching `v*` via `.github/workflows/docker.yml`.

- Every `v*` tag pushes `:<version>`.
- Stable tags matching `v<major>.<minor>.<patch>` also push
  `:<major>.<minor>` and `:latest`.

Required GitHub repo secrets:

- `DOCKERHUB_USERNAME`
- `DOCKERHUB_TOKEN`
