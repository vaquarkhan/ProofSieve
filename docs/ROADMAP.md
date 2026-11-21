# Roadmap

Each release is done when its exit checks pass on a fixture repo in this repository. Dates are sequence, not a promise. v0.1 is about three weeks of focused work. v1.0 is the first release that covers all four gates.

## v0.1 — Stop the bleeding

Scope lock, pre-write guard, CI scope check, complexity caps.

Exit checks:

- A fixture agent write outside `allowed_paths` makes `proofsieve guard` exit 2 and leaves the file unchanged.
- A write to `.proofsieve/scope.lock.yaml` exits 2.
- A patch that adds `it.skip` or deletes an `expect(` in a locked test fails `proofsieve check`.
- A new function with cognitive complexity 11 fails `check`. An untouched legacy function over the cap does not.
- `guard` p99 under 25 ms on the fixture lock, no sockets opened.
- Windows amd64 binary. Hook snippets from `proofsieve init` match current Cursor and Claude Code payload fields.

Not in this release: mutation, model calls, attestation, Docker.

## v0.2 — Churn class

Bugfix diffs that are mostly format and rename fail closed.

Exit check: a fixture bugfix whose token-identical formatting and rename lines exceed 15% exits 1. A feature diff with the same ratio exits 0 when file and line budgets hold.

## v0.3 — Mutation oracle

`proofsieve mutate` for Python (mutmut) and TypeScript (Stryker or diffguard). `test-gen` context builder that refuses to embed function bodies. GitHub Action runs `check` as a required status.

Exit checks:

- A fixture test that asserts `true` while the diff changes a comparison leaves a Tier-1 survivor and exits 1.
- A test that fails on that mutant exits 0 at the default 80% Tier-1 threshold.
- `test-gen` exits 1 when pointed at a file whose body would otherwise be copied.
- Warm-cache mutate under 120 s on the fixture.

## v0.6 — Proof-only review

SARIF ingest, reproduction runner, five-comment cap, evidence artifact.

Exit checks:

- A findings file with ten prose items and one real base-pass/head-fail test publishes one comment.
- A finding whose `test` field is a shell string publishes nothing and does not execute it.
- Semgrep SARIF duplicates are not repeated by the publisher.
- With the model step removed, a clean Tier-0 run still exits 0.

## v1.0 — Ownership

Behavioral diff, risk score, human attestation, linux and darwin binaries, optional minisign on the evidence artifact, OpenTelemetry export.

Exit checks:

- A diff touching the fixture `auth/` path scores at least 60 and exits 1 with no comment.
- The same diff exits 0 after a human-authored comment whose `sha`, `spec`, and `test` resolve.
- An agent-bot comment with the same body does not unlock the gate.
- The attestation is not stored as a blob in the commit it names.

## How to tell it worked

Measure on one pilot repo, 20 merged pull requests before the gate and 20 after v0.1, then again after v0.3. These are targets for the pilot, not a launch claim.

| Signal | Target | When |
| --- | --- | --- |
| Median files changed on pull requests labeled bug | down 60% from the pilot baseline | after v0.1 |
| Locked test files modified without `allow_test_mutation` | 0 merges | v0.1 |
| Inline bot comments per pull request | at most 5, and only after v0.6 turns the publisher on. Until then, 0 from ProofSieve | v0.6 |
| Share of published comments a human applies or explicitly dismisses | above 75% | v0.6 |
| Tier-1 mutation kill rate on agent-authored pull requests | at least 80% | v0.3 |
| High-risk merges with a resolved attestation | 100% of merges over the threshold | v1.0 |

Defect escape and time-to-understand during incidents are worth recording. They are lagging and noisy. They are not a v0.1 exit check.

## Pilot install (after v0.1 builds)

1. Commit `.proofsieve/policy.yaml` with complexity caps and empty critical paths.
2. For one service, write a spec and commit `scope.lock.yaml`.
3. Install the hook via `proofsieve init`. Keep CI required even for people who do not run the hook.
4. Turn off any LLM review bot that comments on the same repo. Run Semgrep in CI as Tier 0.
5. Add mutation only after two weeks of scope-lock data, so a red mutant build is not confused with a scope failure.
