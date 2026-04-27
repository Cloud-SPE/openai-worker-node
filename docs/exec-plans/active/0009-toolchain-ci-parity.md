---
id: 0009
slug: toolchain-ci-parity
title: Toolchain + CI parity bump (golangci-lint v2, action majors, sibling scaffold)
status: active
owner: TBD
opened: 2026-04-27
---

## Goal

Restore CI lint health and modernise the CI scaffold in one bundled PR:

1. Bump `golangci-lint` from `v1.64.8` to `v2.11.4` — the v1.x line ended in early 2025 and the pin is fragile against any local Go ≥ 1.26 (export-data format mismatch).
2. Bump GitHub Action pins to current majors: `actions/checkout@v4 → @v6`, `actions/setup-go@v5 → @v6`, `golangci/golangci-lint-action@v6 → @v9`.
3. Adopt the `livepeer-modules-project` sibling repos' expanded CI structure: split into `lint.yml` + `test.yml`, add `govulncheck` (informational), add `custom-lints` job, add a coverage gate at the `core-beliefs.md` invariant of 75%.
4. Add `.github/dependabot.yml` (gomod + github-actions ecosystems) so this drift doesn't reaccumulate.

## Non-goals

- **Not** bumping Go 1.25 → 1.26. Siblings are still on 1.25.0; defer to a separate cross-repo plan that bumps everything in lockstep.
- **Not** introducing a `.golangci.yml` config — repo currently runs configless and v2 defaults are an acceptable starting baseline. Tightening lints is a follow-up plan.
- **Not** modifying the custom lint surface (`lint/payment-middleware-check`) beyond what golangci-lint v2 mechanically requires.
- **Not** porting CODEOWNERS / PR template / issue templates from siblings — separate repo-metadata plan.
- **Not** touching the sibling `livepeer-modules-project` repos. See cross-repo dependencies.

## Cross-repo dependencies

> **Correction (2026-04-27 — post-execution):** The sibling state described below was misread at planning time. The actual canonical CI for `livepeer-modules-project` lives in the **root** `.github/workflows/ci.yml` (consolidated via their plan 0008), not in the per-module `<module>/.github/workflows/lint.yml` files I originally read. See the Decisions log entry "Cross-repo audit retract" for the corrected state. The plan still executed correctly — the lint pin bump was the right call regardless of sibling state — but the framing of "lead-and-drift on identical pins" was wrong. Siblings already pin `golangci-lint-action@v8` with `version: latest` and have a v2-schema `.golangci.yml` + a Dependabot github-actions ecosystem. The genuine deltas vs siblings are only `actions/setup-go@v5` and `actions/checkout@v4`, both of which their existing Dependabot will handle.

(Original framing, retained for history:)
Sibling `livepeer-modules-project` (payment-daemon, service-registry-daemon, chain-commons, protocol-daemon) currently pin **identical** versions to ours: `go 1.25.0`, `golangci-lint v1.64.8`, `checkout@v4`, `setup-go@v5`, `golangci-lint-action@v6`. The `lint.yml` "match sibling" comment in this repo mirrors theirs verbatim ("v1.64.8 is the minimum that parses Go 1.25 stdlib export data").

**Decision (2026-04-27):** Lead-and-drift. This repo bumps now; the comment is updated to "Diverged from sibling pending modules-project bump (tracked: <issue link TBD>)". A tracking issue against `livepeer-modules-project` is filed as part of this plan's execution. Rationale: lint pin currently risks silent CI breakage on any local Go ≥ 1.26; sibling-coordinated bump is desirable but not blocking; this repo already leads on shipping cadence (just cut v0.8.10 against sibling v1.1.0).

## Current state vs target (verified 2026-04-27)

> **Correction:** The "Siblings" column below reads from the stale per-module workflow files, not the canonical root `ci.yml`. Updated row inline below the original table.

| Component | This repo | Siblings (claimed) | Latest stable | Plan target |
|---|---|---|---|---|
| Go (`go.mod`) | `1.25` | `1.25.0` | `1.26.2` | `1.25` (unchanged — non-goal) |
| Go (Dockerfile) | `golang:1.25-alpine` | `golang:1.25-alpine` | `golang:1.26-alpine` | unchanged |
| Go (CI `go-version`) | `'1.25'` | `'1.25'` | `1.26.x` | unchanged |
| `golangci-lint` | `v1.64.8` | `v1.64.8` | `v2.11.4` | **`v2.11.4`** ⚠ major |
| `actions/checkout` | `@v4` | `@v4` | `v6.0.2` | **`@v6`** ⚠ +2 majors |
| `actions/setup-go` | `@v5` | `@v5` | `v6.4.0` | **`@v6`** ⚠ +1 major |
| `golangci/golangci-lint-action` | `@v6` | `@v6` | `v9.2.0` | **`@v9`** ⚠ +3 majors |
| Dependabot | absent | configured | — | **add** (gomod + github-actions) |
| `lint.yml` jobs | 3 | 6 | — | **add** govulncheck + custom-lints |
| `test.yml` | absent | present (race + coverage) | — | **add** |
| Coverage gate | convention only | enforced ≥75% | — | **enforce ≥75%** |

**Corrected sibling state** (verified post-execution from `livepeer-modules-project/.github/workflows/ci.yml`):

| Component | Siblings (actual) | Delta vs this repo |
|---|---|---|
| `golangci-lint` | `version: latest` (resolves to v2.x) | We pin specifically; they float — philosophical split |
| `golangci-lint-action` | `@v8` | We're one major ahead at `@v9` |
| `actions/setup-go` | `@v5` | Same — both behind `@v6` (their Dependabot will surface) |
| `actions/checkout` | `@v4` | Same — both behind `@v6` (their Dependabot will surface) |
| `.golangci.yml` schema | `version: "2"` (root + 4 module configs) | Same |
| Dependabot | `gomod` (per-module) + `github-actions` (root) | Same scope, predates ours |
| Workflow topology | Single consolidated `ci.yml` (path-filtered) + `docker.yml` | We split into `lint.yml` + `test.yml`; both reasonable |

## Approach

**Bundling decision (2026-04-27):** Single PR containing all four tracks. Rationale: tracks are interrelated (Track 2 depends on Track 1's golangci-lint bump for v2 syntax, and Track 4 depends on Track 1's action versions to know what Dependabot should manage); reviewer can see the full intent in one diff.

### Track 1 — version bumps

1. `.github/workflows/lint.yml`:
   - `golangci/golangci-lint-action@v6` → `@v9`
   - `version: v1.64.8` → `version: v2.11.4`
   - Update the "Match sibling" comment to reflect lead-and-drift state, including the tracking-issue URL.
2. Repo-wide replace: `actions/checkout@v4` → `@v6`, `actions/setup-go@v5` → `@v6`.
3. Run `golangci-lint v2.11.4` locally against `./...`. Triage warnings into three buckets:
   - **Trivial** (≤5 LOC, no behaviour change) → fix in this PR.
   - **Real bugs surfaced** → fix in this PR with a one-line per-fix note in the PR description.
   - **Refactor-scale or judgment-call** → suppress with `//nolint:<linter> // <reason>` and add a single combined entry to `docs/exec-plans/tech-debt-tracker.md` referencing this plan.
4. Bump the implied `golangci-lint` floor in `AGENTS.md` Toolchain section (`v2.11+`).

### Track 2 — CI scaffold parity

1. Split `.github/workflows/lint.yml` jobs:
   - **Keep in `lint.yml`:** `golangci`, `gofmt`, `go-mod-tidy`.
   - **Add to `lint.yml`:** `govulncheck` (continue-on-error: true — stdlib CVEs trail the latest Go patch and are absorbed by toolchain bump; non-stdlib hits get reviewed manually); `custom-lints` (runs `go run ./lint/payment-middleware-check --root .`).
2. Create `.github/workflows/test.yml`:
   - Job `test`: `go test -race -coverprofile=coverage.out -covermode=atomic ./...`
   - Job step: coverage gate at 75% per package with source. Implementation: either port sibling's `lint/coverage-gate/...` analyzer or implement an inline shell check using `go tool cover -func`. Decision deferred to execution.
   - Upload coverage artifact for inspection.
3. Skip the sibling's `proto` job — this repo consumes payment-daemon's generated proto code, doesn't own .proto files.

### Track 3 — Dependabot

1. Create `.github/dependabot.yml`:
   ```yaml
   version: 2
   updates:
     - package-ecosystem: gomod
       directory: /
       schedule:
         interval: weekly
       open-pull-requests-limit: 5
       groups:
         grpc-and-protobuf:
           patterns:
             - "google.golang.org/grpc*"
             - "google.golang.org/protobuf"
         prometheus:
           patterns:
             - "github.com/prometheus/*"
     - package-ecosystem: github-actions
       directory: /
       schedule:
         interval: weekly
       open-pull-requests-limit: 3
   ```
2. Note in commit message: github-actions ecosystem is intentionally added (siblings only manage gomod) — this repo got burned by silent action drift; the cost of weekly PRs is worth the visibility.

### Track 4 — verification & ship

1. Local verification:
   - `make lint` passes
   - `make test` passes (race + coverage gate)
   - Confirm `lint/payment-middleware-check` still runs as part of the new `custom-lints` CI job
2. Open as a single PR; reviewer reads four logical sections.
3. After merge: file the modules-project tracking issue referenced in the lint.yml comment.

## Decisions log

### 2026-04-27 — Cross-repo audit retract: sibling pins were misread
After landing the bundled commit, the modules-project team audited the tracking issue draft and pointed out that the cross-repo claims were wrong on 5 of 7 points. Verified ground truth from `livepeer-modules-project/.github/workflows/ci.yml`:

- They pin `golangci-lint-action@v8` (not `@v6` as planned-against)
- They use `version: latest` (resolves to v2.x — not `v1.64.8`)
- All four `.golangci.yml` declare `version: "2"` (already on v2 schema)
- Their Dependabot already manages `github-actions` ecosystem
- They have a single consolidated `ci.yml` (their plan 0008), not per-module `lint.yml` + `test.yml`

Where the bad data came from: the per-module `<module>/.github/workflows/lint.yml` files in their checkout still pin `v1.64.8` + `@v6`. Those are stale leftovers from before their CI consolidation. I read them at session start and treated them as canonical without checking the root `.github/workflows/`.

Genuine deltas vs siblings (only):
- `actions/setup-go@v5` (one major behind `@v6`)
- `actions/checkout@v4` (two majors behind `@v6`)

Both will surface on their next weekly Dependabot run. **Decision: don't file the tracking issue against modules-project; the premise was wrong.** The lint pin bump itself was still the right call for this repo regardless of sibling state.

Lesson for future cross-repo work: read root-level workflow directories before per-module ones. Per-module workflow files in monorepos are frequently stale leftovers after CI consolidation.

### 2026-04-27 — Lead-and-drift on cross-repo path
Reason: Sibling pin is identical to ours; coordinated bump would be ideal but is not blocking; CI lint is currently fragile against any local Go ≥ 1.26. Comment in `lint.yml` will be updated to acknowledge the temporary drift with a tracking-issue link.

### 2026-04-27 — Bundle as one PR
Reason: The four tracks are interrelated (Track 2 depends on Track 1's v2 syntax; Track 3 depends on Track 1's action versions). Reviewer sees the full intent in one diff. Splitting into 3 sequenced PRs would create temporary inconsistent states between merges.

### 2026-04-27 — Pin to v2.11.4, not floating
Reason: Existing `lint.yml` comment says "Bump with a re-triage commit; don't chase `latest` — silent-breakage risk." v2.11.4 is current stable at planning time. Re-pin happens deliberately when needed, not on every dependabot tick.

### 2026-04-27 — Adopt github-actions Dependabot ecosystem
Reason: This repo just experienced exactly the silent drift this would prevent (`@v6` golangci-lint-action is 3 majors stale). Siblings don't manage actions via Dependabot; this is an intentional lead. Cost: a weekly PR or two; benefit: never quietly accumulate action drift again.

## Open questions

1. **Coverage-gate implementation** — port sibling's `lint/coverage-gate/...` analyzer (creates a Go-tool dependency), or use an inline shell check on `go tool cover -func` output? Defer to execution; either works.
2. **Tracking issue against modules-project** — who files it? Worker-team or modules-project-team? Should be filed as part of execution so the lint.yml comment can reference a real URL.
3. **golangci-lint v2 warning surge** — the actual size of Track 1's bucket-3 (refactor-scale suppressions) is unknowable until the bump runs. If the bucket explodes, consider deferring scaffold tracks (2/3) to a follow-up PR after Track 1 lands as a focused fix.

## Artifacts produced

- One PR titled (proposed): `ci+deps: golangci-lint v2 + action major bumps + sibling-parity CI scaffold`
- Modified files:
  - `.github/workflows/lint.yml`
  - `AGENTS.md` (Toolchain section)
  - Possibly `Makefile` (if a v2-only flag needs adoption)
- New files:
  - `.github/workflows/test.yml`
  - `.github/dependabot.yml`
  - Possibly `lint/coverage-gate/main.go` if porting the sibling's analyzer
- Possibly: one combined entry in `docs/exec-plans/tech-debt-tracker.md` for any deferred lint suppressions

## Risks

- **v1→v2 default-linter delta.** Default linter set differs between v1 and v2; warning surge is unknowable in advance. Mitigation: 3-bucket triage; if bucket-3 explodes, spin Tracks 2+3 into a follow-up PR.
- **Action multi-major skips.** `checkout@v4 → @v6` and `golangci-lint-action@v6 → @v9` each cross multiple majors with breaking changes. Specifically: `golangci-lint-action@v7` changed config schema handling; `@v8` enabled v2 schema by default; `@v9` removed deprecated inputs. Mitigation: read each major's release notes during execution; expect the v6→v9 jump to surface input-name or default-behaviour changes.
- **Sibling drift narrowing.** If siblings bump within ~2 weeks of this landing, our "Diverged" comment goes stale fast. Mitigation: re-check sibling state before merging; update comment if it changes.
- **Coverage gate in CI exposes existing gap.** Adopting a 75% gate as a CI failure (not just a documented invariant) may surface packages currently below 75%. Mitigation: run coverage locally first; if below threshold, decide whether to lift or temporarily exempt with a justification entry.
