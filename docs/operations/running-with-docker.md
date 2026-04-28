# Running with Docker

The `compose.yaml` at the repo root stands up `openai-worker-node`
alongside `livepeer-payment-daemon` (receiver mode), sharing one
`worker.yaml` and one unix socket.

## Prerequisites

- Docker 24+ with Compose v2.

The dev `compose.yaml` pulls the `payment-daemon` sidecar as a published image (`tztcloud/livepeer-payment-daemon`); the worker image builds from this repo alone. No sibling checkout required.

## First run (dev mode, fake broker)

1. Copy the annotated example config:

   ```bash
   cp worker.example.yaml worker.yaml
   ```

2. Edit `worker.yaml`:
   - Set `payment_daemon.broker.mode: fake`.
   - Drop `payment_daemon.broker.rpc_url` and
     `payment_daemon.broker.ticket_broker_contract`.
   - Add `payment_daemon.broker.fake_sender_balances_wei` with the
     bridge's ETH address and a generous balance, e.g.:

     ```yaml
     fake_sender_balances_wei:
       "0xBRIDGE_ADDR_HERE": "1000000000000000000"
     ```

   - Replace `recipient_eth_address` with a real-looking address
     (the format check requires 0x + 40 hex chars, but the value
     doesn't need to be a real wallet in fake mode).
   - Set `payment_daemon.keystore.path` to something writable inside
     the container, or point at a pre-made keystore volume if you've
     created one. Dev mode usually works without a keystore by using
     fake-mode broker which skips signature operations entirely; see
     the library's `running-the-daemon.md` for the full keystore
     contract.

3. Bring the stack up:

   ```bash
   docker compose up --build
   ```

   The first build takes 1–2 minutes (module download + two Go
   builds). Subsequent builds are fast thanks to layer caching.

4. Verify:

   ```bash
   curl -s http://localhost:8080/health | jq
   # {"status":"ok","protocol_version":1,"max_concurrent":32}

   curl -s http://localhost:8080/capabilities | jq
   # {"protocol_version":1,"capabilities":[...]}
   ```

## Production mode (real broker)

1. Keep `broker.mode: ethereum` in `worker.yaml`.
2. Point `rpc_url` at your JSON-RPC endpoint and set
   `ticket_broker_contract` to the `TicketBroker` contract address for
   the chain you're deploying to.
3. Mount a real V3 JSON keystore into the `payment-daemon` container
   and set `keystore.path` + `keystore.passphrase_env` to match.
4. Make sure every `capabilities[].models[].backend_url` points at
   an actual inference server the worker can reach.

## How the shared YAML works

The `worker.yaml` file is bind-mounted into BOTH containers at
`/etc/livepeer/worker.yaml` via a read-only volume. Both processes
parse it independently at startup; on mismatch (drift between the
file they see and what they expect the peer to see) the worker
refuses to start.

The worker's startup cross-checks further by calling
`PayeeDaemon.ListCapabilities` over the unix socket and asserting
byte-equality with its own parse for protocol_version + every
(capability, model, price) triple. Flipping
`worker.verify_daemon_consistency_on_start: false` disables this
check — only do so in dev where you know you're out of lockstep.

## Log level

`command: [--log-level=info]` in `compose.yaml` is the default. Valid
values: `error`, `warn`, `info`, `debug`. Debug logs include per-
request payment details (sender, work_id, work_units) that are too
chatty for production but invaluable during integration.

## Observability

The worker can expose Prometheus metrics on a separate listener. Off
by default — enable by setting `METRICS_PORT` in `.env` (or passing
`--metrics-listen=:9093` directly when running the binary outside
Docker).

Two flags control the listener:

| Flag | Default | Notes |
|---|---|---|
| `--metrics-listen` | `""` (off) | `host:port`. Empty = no listener. Worker port allocation is `:9093` per [`livepeer-modules-conventions/port-allocation.md`](../../../livepeer-modules-conventions/port-allocation.md). |
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
`METRICS_PORT`:

```yaml
ports:
  - '127.0.0.1:${METRICS_PORT}:${METRICS_PORT}'
```

Bind to `127.0.0.1` if your Prometheus runs on the same VPS;
drop the prefix only if you've firewalled the port at the network
level. The endpoint is unauthenticated — `/metrics` enumerates every
configured `(capability, model)` pair, which is operationally
sensitive.

## Upgrading

To pick up a new payment-daemon release, bump `DAEMON_TAG` (or the pinned `image: tztcloud/livepeer-payment-daemon:vX.Y.Z` in `compose.prod.yaml`) and run:

```
docker compose pull && docker compose up -d
```

Schema or proto changes from the daemon side land here as PRs that update `internal/config/` (worker.yaml schema) and/or `internal/proto/livepeer/payments/v1/` (regenerate with `make proto`). Drift between this repo's pinned proto/schema and a running daemon is caught at startup by `VerifyDaemonCatalog` — the worker refuses to start on mismatch rather than serving wrong-priced requests.
