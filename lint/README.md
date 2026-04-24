# lint/

Custom lints that enforce architectural invariants beyond what `golangci-lint` handles. Each lint:

- Lives in its own subdirectory.
- Is a Go program invokable via `go run ./lint/<name>/...`.
- Produces structured errors with **remediation instructions** embedded in the message.

## Planned lints

### layer-check

Enforces the dependency rule from `docs/design-docs/architecture.md`:

```
types → config → repo → service → runtime
```

plus `providers/` accessible from all, and `service/modules/<name>/` subject to the same rule internally.

**Status: unimplemented.** Placeholder. Initial enforcement will lean on golangci-lint's `depguard` once 0002-chat-completions-module gives us real import patterns to constrain. Full AST pass opens as a dedicated plan if depguard proves insufficient.

**Tracked:** `tech-debt-tracker.md` entry `layer-check-full-impl`.

### payment-middleware-check

Enforces core belief #3: every paid HTTP route passes through `runtime/http.RegisterPaidRoute`, never `Register` alone. This is the mechanical check that substitutes for "remember to validate payment" discipline.

Inspection target: the call sites in `internal/runtime/http/register.go` (and anywhere else calling the HTTP mux's `Register*` methods). For each registered path matching a capability URI (`/v1/...`), assert the call uses `RegisterPaidRoute`.

**Status: unimplemented.** Placeholder. Targeted for a plan after 0003-payment-middleware lands the actual middleware; the lint has nothing to check until then.

**Tracked:** `tech-debt-tracker.md` entry `payment-middleware-check-impl`.

### no-raw-log

Rejects `fmt.Println`, `log.Print*` in favor of slog. Delivered via golangci-lint's `forbidigo` rule — not a custom Go program.

**Status: scheduled** with the first `.golangci.yml` landing.

### no-secrets-in-logs

AST-based analyzer. Walks every non-test `.go` file, finds `slog.*` calls with literal attr keys matching a deny-list (`password`, `passphrase`, `secret`, `apikey`, `keystore`, `mnemonic`, `authtoken`). Ports the library's implementation — same deny-list, same `//nolint:nosecrets` escape hatch.

**Status: unimplemented.** Will port from the library once relevant code exists.

### doc-gardener

Validates frontmatter + internal links across `docs/design-docs/*.md` and `docs/exec-plans/{active,completed}/*.md`. Checks required frontmatter keys, status consistency with directory, resolvable internal links.

**Status: unimplemented.** Will port from the library once meaningfully populated.

## Format

Lint errors must include:

```
<file>:<line>: <rule-id>: <one-line problem>
  Remediation: <one-or-two sentence guidance>
  See: docs/design-docs/<relevant-doc>.md
```

This lets agents fix violations autonomously from the error message.
