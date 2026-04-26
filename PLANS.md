# PLANS — how work is planned in this repo

Plans are first-class artifacts. They are versioned in-repo alongside code so agents can read progress and decision history from the repository itself.

This repo follows the same planning convention as its siblings (`livepeer-modules-project/payment-daemon`, `openai-livepeer-bridge`). The sections below are intentionally mirror-copies; differences are called out explicitly.

## Two kinds of plans

### Ephemeral plans

For small, self-contained changes (< ~50 LOC, single domain, no schema/protocol change). Written inline in the PR description. No file created.

### Exec-plans

For complex work: multi-domain, new capability module, schema/protocol change, or anything an agent might pause mid-implementation and resume on later. Lives in `docs/exec-plans/active/`.

## Exec-plan file layout

```
docs/exec-plans/active/
├── 0001-<slug>.md      # in-flight
├── 0002-<slug>.md      # in-flight
docs/exec-plans/completed/
├── 0001-<slug>.md      # archived on merge
docs/exec-plans/tech-debt-tracker.md
```

IDs are monotonic, zero-padded to 4 digits, unique within this repo. Cross-repo coordination (e.g. this repo's `0001` depending on `livepeer-modules-project/payment-daemon`'s `0018`) is captured in the `## Cross-repo dependencies` section of a plan.

## Exec-plan template

```markdown
---
id: 0001
slug: repo-scaffold
title: Stand up repo scaffolding
status: active          # active | blocked | completed | abandoned
owner: <agent-or-human>
opened: YYYY-MM-DD
---

## Goal
One paragraph. What are we trying to achieve and why.

## Non-goals
What is explicitly NOT in this plan.

## Cross-repo dependencies
Plans in sibling repos this one needs completed first (or in-flight with
guaranteed order). Omit section when none.

## Approach
Bullet list of steps. Check off as completed.

- [ ] Step 1
- [ ] Step 2

## Decisions log
Append-only. Each decision: date + one-paragraph rationale.

### YYYY-MM-DD — <short title>
Reason: …

## Open questions
Things we need to answer before or during implementation.

## Artifacts produced
Links to PRs, generated docs, schemas created.
```

## Lifecycle

1. **Opened** — file created in `active/`, status `active`.
2. **In progress** — steps checked off, decisions appended.
3. **Blocked** — status flipped to `blocked`, open-questions populated, escalated.
4. **Completed** — all steps checked; file moved from `active/` → `completed/`, status updated, final artifacts linked.
5. **Abandoned** — status flipped to `abandoned`, reason added to decisions log, file moved to `completed/`.

## Capability-module plans

Each new capability module (`chat_completions`, `embeddings`, `images`, `audio_speech`, `audio_transcriptions`, `video_generations`) is its own exec-plan. Modules are additive; a plan lands when its module is on-parity with the bridge's matching route handler. Module plans all share a consistent shape:

1. Register the capability in the module registry.
2. Implement the request/response schemas at the boundary.
3. Implement the work-unit estimator.
4. Implement the backend client and dispatch.
5. Implement metering + reconciliation.
6. Integration test against a fake backend.

A module is not "done" until its integration test drives a request end-to-end through the payment middleware with a fake broker.

## Rules

- Never modify plans in `completed/`. History is immutable.
- Every PR that changes `internal/` must link to an exec-plan in its description (unless the change is ephemeral).
- Plans may reference design-docs; design-docs may not reference plans.
- `tech-debt-tracker.md` is append-only with strike-through when resolved.
- A plan in this repo that depends on a plan in a sibling repo must name the sibling plan in its `## Cross-repo dependencies` section.
