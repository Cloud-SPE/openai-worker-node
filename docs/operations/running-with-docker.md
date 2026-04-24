# Running with Docker

The `compose.yaml` at the repo root stands up `openai-worker-node`
alongside `livepeer-payment-daemon` (receiver mode), sharing one
`worker.yaml` and one unix socket.

## Prerequisites

- Docker 24+ with Compose v2.
- A sibling checkout of `livepeer-payment-library` (required until the
  library tags a release; the replace directive in `go.mod` and the
  `additional_contexts: library: ../livepeer-payment-library` in
  `compose.yaml` assume it's at `../livepeer-payment-library`).

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

## Upgrading

When the library tags a release:

1. Bump `require github.com/Cloud-SPE/livepeer-payment-library v…`
   in `go.mod`; remove the `replace` directive.
2. Remove `additional_contexts: library: …` from `compose.yaml`.
3. `docker compose build --no-cache && docker compose up -d`.
