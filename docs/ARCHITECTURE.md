# Architecture

ProofSieve sits in two places. The workstation stops an out-of-scope write while the agent is still in the turn. The pull request recomputes the same policy from git, where the agent cannot edit the result.

```
PLANE 1 — workstation (best-effort, same turn)
  Cursor / Claude Code tool call
        │  stdin JSON (tool name, path, patch or command)
        ▼
  proofsieve guard  ── reads .proofsieve/scope.lock.yaml
        │              and .proofsieve/policy.yaml
        ├─ exit 0  tool runs
        └─ exit 2  tool does not run; agent receives the violation

PLANE 2 — pull request (authoritative)
  proofsieve check
        ├─ Gate 1  scope, churn, complexity, test locks
        ├─ Gate 2  oracle unchanged + incremental mutation
        ├─ Gate 3  SARIF, then proof-only review comments
        └─ Gate 4  behavioral diff + attestation when risk is high
  exit 0 pass · exit 1 policy fail · exit 2 runner/infra error
```

Plane 1 and plane 2 share one policy evaluator. A path allowed by the hook and rejected by CI is a bug.

## Gate 1 — scope lock

Before implementation, a person approves a spec (GitHub Spec Kit or OpenSpec). `proofsieve lock` compiles that approval into `.proofsieve/scope.lock.yaml`:

- `allowed_paths` and `denied_paths`
- `max_files`, `max_loc_delta`
- SHA-256 of human-owned oracle and regression tests
- `change_type`: `bugfix`, `feature`, or `refactor`

The lock file, `policy.yaml`, and the oracle tree are on the deny list. The agent cannot widen its own budget. A human edits the lock, or runs `proofsieve lock` outside the agent session.

The guard checks, in order:

1. Normalized relative path is under an allow glob and under no deny glob.
2. The write would not push the session over `max_files` or `max_loc_delta`. The session counter is a local file under `.proofsieve/run/`, ignored by git.
3. If the path is locked, the new bytes must hash to the recorded SHA-256. Skip annotations and deleted assertions fail even when the hash check is the one that fires.
4. Shell and apply-patch commands are denied when they name a denied path or a locked test. This is a pattern check, not a sandbox. It will miss obfuscated commands. CI catches those.

CI repeats the check from `git diff base...head`, so a bypassed hook still fails the required check. For `change_type: bugfix`, CI also classifies hunks:

| Class | Definition |
| --- | --- |
| formatting | Tree-sitter token stream is identical once trivia and comments are removed |
| rename | Identifier replacements only, no structural change |
| test_weakening | Removed assertion, added skip/disable, or deleted test |
| functional | Everything else |

Formatting plus rename lines above `max_nonfunctional_churn_pct` (default 15) fail a bugfix. Feature and refactor scopes do not use that ratio. They use their own file and line budgets.

## Gate 2 — test oracle

The oracle is a set of tests a human committed, addressed by path and SHA-256. The coding agent can read them. It cannot modify them unless the lock sets `allow_test_mutation: true`, and that flag is itself inside the denied `.proofsieve/` tree.

Line coverage is recorded and never gates.

Mutation runs only on lines the diff changed, and only on operators that can hide a logic bug:

| Tier | Operators | CI |
| --- | --- | --- |
| 1 logic | conditional boundary, negated condition, math operator, return-value replacement | Fail under the configured kill rate (default 80%) |
| 2 semantic | boolean substitution, increment/decrement swap | Warn under 70% |
| 3 observability | statement deletion, branch removal on log and metric calls | Report only |

Surviving Tier-1 mutants are written to `surviving_mutants.json` (file, line, operator, original, mutated). That file is the feedback to a person or a test-writing session. A green suite that leaves a Tier-1 mutant alive fails the gate.

### Clean-room test drafts

`proofsieve test-gen` is an optional context builder, not a gate. Its prompt is allowed to contain:

- acceptance criteria from the linked spec
- public signatures, OpenAPI, and SQL schemas extracted by the parser
- fixtures

Implementation bodies are replaced with signatures before the prompt is assembled. The builder refuses to run if it cannot strip them. Drafts land in a candidate directory. They become oracle tests only when a human moves them and `proofsieve lock` records the new digest. Until then they are untrusted, including when they pass.

Hidden tests follow the same rule: a directory on `denied_paths`, executed in CI, absent from the agent's writable set.

## Gate 3 — review triage

Default posture: no model posts a review.

Tier 0 runs local tools and reads SARIF 2.1.0: Semgrep (`semgrep scan`, not the hosted platform), the language linter, the type checker, and an architecture rule pack (dependency-cruiser or ArchUnit). Findings in those classes are the comment source for security and structure. A later model pass is forbidden from repeating a rule id Tier 0 already owns.

Tier 1 accepts a finding only as declarative JSON:

```json
{
  "claim": "half-up rounding returns 0.3 for 1.25 at scale 1",
  "test": "pricing/rounding_test.go:TestHalfUp",
  "expect": { "base": "pass", "head": "fail" }
}
```

The runner executes that test name in two worktrees, base and head, with `shell` disabled, network disabled, and a 30-second cap. A finding with no test name, or a request to run a shell string, is dropped. Optional third worktree: a proposed patch must make the test pass. Dropped findings go to the job log.

Tier 2 keeps at most `max_inline_comments` (default 5), ordered by severity then by whether the reproduction is deterministic. Dismissed fingerprints (rule id + normalized node hash + path) live in `.proofsieve/ignore.json`. A different model must not re-open them.

If the model call times out or returns malformed JSON, Gate 3 reports Tier 0 only and does not pass a fake "no findings" review. Exit code stays 0 when Tier 0 is clean, 1 when Tier 0 failed.

## Gate 4 — human ownership

`proofsieve check` writes a behavioral diff from the AST, not from a model summary:

- exported functions and types added, removed, or signature-changed
- new I/O, SQL, or permission-shaped calls (configured name list)
- callers inside the repo whose callees changed, when the language plugin can resolve them
- spec ids cited by the lock

Complexity caps apply to functions the diff added or edited: cognitive complexity ≤ 10, nesting ≤ 4, length ≤ 50 lines. Cognitive complexity is the cap (nesting is what makes agent output unreadable). Cyclomatic complexity is not a second cap.

### Risk score

Each term is in `[0, 1]`.

```
F = min(1, files_changed / max_files)
C = min(1, added_cognitive_complexity / 20)
P = 1 if any changed path matches critical_paths, else 0
K = 0 if mutation is disabled
    1 if mutation is enabled and the score is missing
    1 - tier1_kill_ratio otherwise

R = min(100, round(100 * (0.25*F + 0.20*C + 0.35*P + 0.20*K)))
```

Default threshold is 60. At or above it, merge stays blocked until a comment from the pull-request author (a human account, not the agent bot) contains:

```
/proofsieve attest
sha: <head sha>
spec: <id that exists in the linked spec>
invariant: <one sentence>
failure: <one sentence, rollback included>
test: <test name that exists on head and is not skipped>
```

The workflow checks the sha, the spec id, and the test name. It does not grade the English. A control-flow quiz is off unless `require_diff_quiz: true`.

The attestation lives in the pull-request comment, not in the commit. Putting it in the tree would change the sha it attests.

## Evidence

Each CI run uploads `.proofsieve/evidence/<head-sha>.json` as a job artifact: gate verdicts, tool versions, policy hash, mutation summary, and attestation comment url. GitHub's required check is already bound to the sha. A new commit needs a new run. v1.0 may sign the artifact with minisign. v0.x does not claim a signature it does not verify.

## Runtime

| Piece | Choice |
| --- | --- |
| CLI and guard | Go, static binary, Windows amd64 first (this project's dev machine), then linux amd64/arm64 and darwin arm64 |
| Parse and complexity | Tree-sitter via go-tree-sitter. v0.1 grammars: TypeScript and Python. Go, Java, Rust follow the mutation adapters |
| Policy | YAML decoded into Go structs, validated by the JSON Schemas in `schemas/`. OPA is not on the hook path |
| Sandbox | Rootless container, `--network=none`, read-only root, no shell entrypoint. Used by Gate 3 only |
| Specs | Read-only input. Spec Kit or OpenSpec stays the authoring tool |

## Threat model (v0.1)

| Attack | Control |
| --- | --- |
| Agent edits a file outside the lock with the Write tool | Hook deny, exit 2 |
| Agent edits `.proofsieve/scope.lock.yaml` to raise `max_files` | Deny list includes `.proofsieve/**` |
| Agent deletes an assertion or adds `pytest.mark.skip` / `it.skip` / `@Disabled` | Locked hash plus a tree-sitter weakening check |
| Agent writes through the shell | Pattern deny in the shell hook; CI diff is authoritative |
| Agent disables the hook | Required CI check |
| Review bot files 100 comments | Publisher only emits verified rows, capped |
| Test generator copies the implementation's bug into the assertion | Generator never sees bodies; mutation on the diff is what fails the build |
| Attestation is a rubber stamp from the agent | Comment author must be the human PR author; `test:` must resolve |

Not claimed: the shell pattern list stops every bypass. Not claimed: mutation proves the spec. Mutation proves the tests notice the edits. The spec link and the human attestation are what bind behavior to intent.
