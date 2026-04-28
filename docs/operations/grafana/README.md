# Grafana dashboard

Pre-built dashboard covering everything `openai-worker-node` emits to Prometheus. Mirrors the metric catalog in [`docs/design-docs/metrics.md`](../../design-docs/metrics.md) and follows the same pattern as the [Cloud-SPE/livepeer-modules `service-registry-daemon` dashboard](https://github.com/Cloud-SPE/livepeer-modules/tree/main/service-registry-daemon/docs/operations/grafana).

## Files

- [`livepeer-openai-worker.json`](livepeer-openai-worker.json) — dashboard definition (Grafana 10.0+, schema 39).

## Import

### UI (one-shot)

1. Grafana -> **Dashboards -> Import**.
2. Upload `livepeer-openai-worker.json` (or paste the contents).
3. When prompted, pick your Prometheus datasource (the one scraping `:9093`).
4. Click **Import**. The dashboard's `uid` is `livepeer-openai-worker` -- re-imports update in place.

### API (CI / GitOps)

```sh
curl -s -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $GRAFANA_TOKEN" \
  -d @<(jq '{dashboard: ., overwrite: true}' docs/operations/grafana/livepeer-openai-worker.json) \
  https://grafana.example.com/api/dashboards/db
```

### Provisioning (file-based)

Drop the JSON into your provisioned-dashboards folder, e.g. `/etc/grafana/provisioning/dashboards/livepeer/`, alongside a `dashboards.yaml`:

```yaml
apiVersion: 1
providers:
  - name: livepeer
    orgId: 1
    folder: Livepeer
    type: file
    options:
      path: /etc/grafana/provisioning/dashboards/livepeer
```

## Layout

8 rows, top-to-bottom in order of operational importance. The last row (`Process + Go runtime`) is collapsed by default.

| Row | What it shows |
|---|---|
| **Overview** | Build version, uptime, total request qps, error rate (color-coded gauge), concurrency utilization (gauge vs. `max_concurrent`). |
| **Request lifecycle** | qps stacked by capability+outcome, p99 latency by capability, top-10 worst (capability, model) p50/p95/p99 table, capacity rejections (painted **red**). |
| **Work units** (the revenue signal) | Work-units rate by capability, by capability+unit table (rate-per-min and total-over-range), top-10 (capability, model) by 1h work-units rate. The bridge joins these against revenue for margin. |
| **Backend health** | Backend qps by capability+outcome (color-coded), backend p99 by capability+model, errors stacked by `error_class`, time-since-last-success table (red >=5min). |
| **Daemon RPC** (unix-socket fast path) | RPC qps by method+outcome, error rate stat, p99 from the **fast histogram** (sub-ms buckets, the unix-socket fast path), p99 from the **default histogram** (pathological tails). |
| **Payment validation** | 402 rejection rate stacked by `reason` (per-reason colors), 1h total stat. |
| **Tokenizer** | Calls by model+outcome -- `fallback` painted yellow, `error` painted red. |
| **Process + Go runtime** (collapsed) | Goroutines, heap-in-use, CPU seconds/sec, open FDs. |

## Variables

The dashboard exposes three template variables at the top:

| Variable | Source | What it does |
|---|---|---|
| `datasource` | Prometheus picker | Switch dashboards across datasources without editing JSON. |
| `job` | `label_values(livepeer_worker_build_info, job)` | Filter to a specific Prometheus scrape job (default: All). |
| `instance` | `label_values(...{job=~"$job"}, instance)` | Filter to a specific worker instance (default: All). |

Every panel's PromQL filters by `{job=~"$job"}`. Multi-instance and multi-job environments work without panel-by-panel surgery.

## Customizing

**Adjust thresholds.** The threshold-driven panels are:
- *Error rate (5m)* gauge -- green/yellow/red at `0`, `0.01`, `0.05` (1%, 5%). Edit per your SLO.
- *Concurrency utilization* gauge -- green/yellow/red at `0`, `0.7`, `0.9`. Tune to your headroom budget.
- *Daemon RPC error rate* stat -- green/yellow/red at `0`, `0.005`, `0.02`.
- *Time since last successful backend response* table cells -- green/yellow/red at `0`, `60s`, `300s`. Should track your slowest backend's expected idle interval.
- *Total payment rejections (1h)* stat -- green/yellow/red at `0`, `10`, `100`.

**Add panels.** Every metric in [`metrics.md`](../../design-docs/metrics.md) has stable label values; copy any panel and tweak the PromQL. The Phase 2 metrics (streaming TTFT, reconcile-delta) will slot in next to existing ones with the same conventions.

**Drop panels.** A worker that never serves a particular capability won't generate `(capability=X)` series; the corresponding rows just show "No data". Either delete them or filter via the `job` variable.

## Pairing with alerts

The dashboard displays metrics; it does NOT ship alert rules. Recommended alerts to wire into Alertmanager once Phase 1 metrics land:

- Error rate >5% sustained for 10 min.
- Concurrency utilization >=90% sustained for 5 min (sized wrong, or backend slow).
- `livepeer_worker_capacity_rejections_total` rate >0 for any capability.
- `(time() - livepeer_worker_backend_last_success_timestamp_seconds)` >=300 (backend stale).
- Daemon RPC p99 (fast histogram) >=5ms (unix-socket degraded).
- `livepeer_worker_payment_rejections_total{reason="process_payment_failed"}` rate >0 (daemon broken).

## Compatibility

- Grafana 10.0+ (uses `schemaVersion: 39`, `timeseries` panel type, `cellOptions` color-background).
- Prometheus 2.x or compatible (Mimir / Cortex / Thanos via `prometheus` datasource plugin).
- Worker version that exposes the Phase 1 `livepeer_worker_*` metric namespace (per [`metrics.md`](../../design-docs/metrics.md)).

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| Panels show "No data" | The worker isn't started with `--metrics-listen`, or Prometheus isn't scraping the host:port (default `:9093`). Check `/metrics` directly with curl. |
| `Build version` stat shows nothing | Worker doesn't yet expose `livepeer_worker_build_info`, or `--metrics-listen` is unset (Recorder is the noop and serves 404). |
| `Capacity rejections` always at 0 | Desired state. Configure an alert to page when the rate goes non-zero. |
| `Time since last successful backend response` rows missing | Worker has not yet completed any successful backend call for that (capability, model). Will populate on first success. |
| `Top 10 (capability, model)` table empty | No work units billed in the last hour. Either no traffic or the modules haven't been wired to call `AddWorkUnits`. |
