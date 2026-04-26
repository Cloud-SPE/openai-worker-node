---
title: Metering & reconciliation
status: accepted
last-reviewed: 2026-04-25
---

# Metering & reconciliation

How the worker translates a request into a `work_units` count for the daemon's `DebitBalance`, and how it reconciles the upfront estimate against the actual cost when the response arrives.

## Work-unit dimensions

Each capability declares one work unit (carried in the `worker.yaml` `work_unit` field, mirrored on the daemon's `CapabilityEntry`). The closed set, with the module that uses it:

| Work unit               | Capability                          | Source-of-truth                                      |
| ----------------------- | ----------------------------------- | ---------------------------------------------------- |
| `token`                 | chat completions, embeddings        | Tokenizer provider (`internal/providers/tokenizer`)  |
| `image_step_megapixel`  | images generations, images edits    | Dimensional formula (n × steps × W × H / 1e6)        |
| `character`             | audio speech                        | `utf8.RuneCountInString(input)`                      |
| `audio_second`          | audio transcriptions                | Backend `duration` field (rounded up)                |

Adding a new dimension requires updating both the daemon's `sharedyaml.Validate` (closed set) and any new tooling that interprets the unit downstream.

## Three timing points

```
                     │  request arrives       response complete
                     │       │                       │
─────────────────────▼───────▼───────────────────────▼─────────►  time
                                                          
   estimate ─────────► EstimateWorkUnits(body, model)
                                                              
   debit-1   ─────────► DebitBalance(sender, work_id, estimate)
                                                              
   serve     ─────────────────────────────────► module.Serve(...) → actualUnits
                                                              
   debit-2   ──────────────────────────────────────────────► DebitBalance(sender, work_id, delta)
                                                                               only when actual > estimate
```

The estimate is the upfront reservation; debit-1 happens BEFORE the backend is dispatched, so a request that would push the customer's balance negative is rejected before any inference work starts. Debit-2 (reconciliation) covers the case where the backend's actual usage exceeded our reservation.

## Why over-debit, not refund

The worker's policy is over-debit-accepted: an estimate that turns out too high is left as the final charge — no refund debit. Reasons:

- **Single-direction reconciliation simplifies the daemon's invariants.** The library's `nonces`/`balance` math assumes monotonically-decreasing balance. Refunds would require either a credit primitive (which doesn't exist on `PayeeDaemon`) or a per-session reconciling ledger we'd have to maintain ourselves.
- **The bridge handles customer-facing refunds.** Customers pay USD; the bridge is responsible for the USD-side reconciliation. The worker→daemon side is wei accounting only and never customer-visible.
- **Estimates are designed to converge.** Token-based estimates are tight (usually within 5%); image megapixels are exact pre-render; audio characters are exact. Audio seconds are the only intentionally loose dimension and we ceiling to one hour by default.

## Per-capability estimation strategies

### Tokens (chat, embeddings)

- Chat: input = `Σ tokenize(role) + tokenize(content)`; output = `max(max_tokens, 2048)`. Estimate = input + output.
- Embeddings: walk the `input` field shape (string, string[], int[], int[][]) and sum tokens / array lengths.

Reconciliation reads `usage.total_tokens` from the backend; falls back to 0 (= no over-debit) when usage is missing.

### Image-step-megapixels (images_generations, images_edits)

`ceil((n × steps × W × H) / 1_000_000)`. Pixels are user-facing megapixels (×1e6, not ×2^20).

No reconciliation: image backends don't emit `usage`. The estimate stands as the final charge.

### Characters (audio_speech)

`utf8.RuneCountInString(input)` — exact, computed before the backend is dispatched. Estimate = actual; no reconciliation.

### Audio-seconds (audio_transcriptions)

The estimate is `MaxAudioSecondsCeil` (default 3600). The backend's `verbose_json` response carries a `duration` field; the module rounds up to the nearest second and returns. The middleware then issues a second `DebitBalance(actual − estimate)` if the actual was higher (rare, since 3600 is generous), or accepts the over-debit if lower.

## Failure paths

- **EstimateWorkUnits returns an error** → 400 invalid_request, no debit.
- **DebitBalance(estimate) errors** → 502 backend_unavailable, no second attempt.
- **DebitBalance(estimate) returns negative balance** → 402 insufficient_balance, no module dispatch.
- **module.Serve errors after writing partial response** → middleware logs, returns. The customer paid for the partial response.
- **Reconciliation debit errors** → middleware logs only — the response is already on the wire and the customer has it.

The "we already wrote bytes" case is why streaming modules (chat completions in stream mode, audio speech) cannot retry: once we've sent a header to the bridge, we own that connection through to a clean (or messy) close.

## Cross-references

- Module contract: [capability-modules.md](capability-modules.md).
- Daemon-side debit semantics: [`../../../livepeer-modules-project/payment-daemon/docs/design-docs/redemption-loop.md`](../../../livepeer-modules-project/payment-daemon/docs/design-docs/redemption-loop.md).
- Bridge customer-facing reconciliation (USD side): [`../../../openai-livepeer-bridge/docs/design-docs/pricing-model.md`](../../../openai-livepeer-bridge/docs/design-docs/pricing-model.md).
