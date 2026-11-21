# Requirements

Requirements below are the contract for implementation. A requirement is in force at the release named in **Release**. Earlier releases must not pretend to satisfy a later one.

Identifiers are stable. Do not renumber them.

## Functional

### FR-1 Scope contract (v0.1)

**FR-1.1** `proofsieve lock --spec <path>` writes `.proofsieve/scope.lock.yaml` valid against `schemas/scope.lock.schema.json`. It records allow and deny globs, file and line budgets, change type, spec path, and SHA-256 of every file matched by `test_integrity.locked_globs`. Default deny always includes `.proofsieve/**`.

**FR-1.2** `proofsieve guard` reads a Cursor or Claude Code hook payload on stdin. For `Write`, `Edit`, `Delete`, and their Claude names, it resolves `tool_input.file_path` or `tool_input.path` to a repo-relative slash path. A path outside `allowed_paths`, inside `denied_paths`, over budget, or colliding with a locked digest exits `2` with a JSON deny body that names the rule. A clean path exits `0`.

**FR-1.3** The guard also runs on shell tool events. It denies the command when the command string contains a denied or locked path, or a write-shaped invocation (`git apply`, `patch `, `tee `, output redirection onto a source path). Obfuscated commands are out of scope for the hook and in scope for FR-1.5.

**FR-1.4** Session budgets count files and added lines across the agent turn using `.proofsieve/run/session.json`. That directory is gitignored. CI does not trust it. CI counts `git diff --numstat base...head`.

**FR-1.5** `proofsieve check --gate scope` exits `1` when the diff violates the lock: extra file, deny hit, line budget, locked-file digest mismatch, or a test-weakening pattern (`skip`, `xit`, `xdescribe`, `pytest.mark.skip`, `unittest.skip`, `@Disabled`, `@Ignore`, removed `assert` / `expect(` / `require(` on a pre-existing test file). `allow_test_mutation: true` in the lock suppresses weakening failures. The agent cannot set that flag without editing a denied path.

**FR-1.6** (v0.2) On `change_type: bugfix`, `proofsieve check` classifies hunks as formatting, rename, test_weakening, or functional using tree-sitter, as defined in the architecture. Non-functional lines above `max_nonfunctional_churn_pct` exit `1`.

### FR-2 Test oracle (v0.3, except the lock which is v0.1)

**FR-2.1** Locked oracle files are enforced in v0.1 by FR-1.5. v0.3 adds mutation.

**FR-2.2** `proofsieve test-gen --spec <path> --out <dir>` builds a context bundle that contains acceptance text, extracted public signatures, and fixtures. It exits `1` if any implementation function body would be copied into the bundle. It does not run tests and it does not update the lock.

**FR-2.3** `proofsieve mutate --base <ref> --head <ref>` maps diff lines to mutants and runs the adapter for the detected language:

| Language | Adapter | Release |
| --- | --- | --- |
| Python | mutmut, diff-scoped | v0.3 |
| TypeScript / JavaScript | Stryker incremental, or diffguard when the repo is already on it | v0.3 |
| Go / Rust | diffguard | v0.3 |
| Java / Kotlin | PITest with history | v0.6 |

**FR-2.4** Tier-1 kill rate below `mutation_gate.min_kill_rate_pct` (default 80) exits `1`. Tier-3 mutants never fail the build. Survivors are written to `surviving_mutants.json` with path, line, operator, original text, and mutated text.

### FR-3 Review (v0.6)

**FR-3.1** `proofsieve check --gate review` reads SARIF 2.1.0 files listed in policy. Findings whose rule id is in the Tier-0 set are emitted as the deterministic review. Duplicate locations collapse.

**FR-3.2** A candidate model finding is a JSON object with `claim`, `test`, and `expect.base` / `expect.head`. The runner executes that test in a base worktree and a head worktree. The finding is kept only when actual results match `expect`. Anything else is logged and omitted. The runner does not invoke a shell string from the finding.

**FR-3.3** At most `review_triage.max_inline_comments` (default 5) inline comments are published, highest severity first. Fingerprints listed in `.proofsieve/ignore.json` are omitted.

**FR-3.4** Model timeout or invalid JSON leaves the Tier-0 verdict unchanged. The gate must not exit `0` because a model said so, and must not exit `1` because a model was absent.

### FR-4 Ownership (v1.0, complexity caps in v0.1)

**FR-4.1** (v0.1) Functions added or modified by the diff fail `proofsieve check` when cognitive complexity exceeds 10, nesting exceeds 4, or length exceeds 50 lines. Defaults are overridable in `policy.yaml`. Limits apply to new and edited functions, not to untouched legacy code.

**FR-4.2** (v1.0) `proofsieve check` writes a behavioral diff: exported signature changes, configured sensitive calls, in-repo callers, and the spec ref. The summary is produced from the parse tree.

**FR-4.3** (v1.0) Risk `R` is computed with the formula and weights in the architecture. When `R` is greater than or equal to `comprehension_gate.risk_threshold` (default 60), the check exits `1` until a pull-request comment by the human author matches the attestation grammar and its `sha`, `spec`, and `test` fields resolve.

**FR-4.4** (v1.0, off by default) `require_diff_quiz` adds two multiple-choice questions whose correct answers are facts from the AST (which branch runs, which function is called). The quiz is skipped when the flag is false.

### FR-5 Evidence (v0.6 artifact, v1.0 attestation link)

**FR-5.1** Every `proofsieve check` run writes `evidence.json` for that head sha: policy hash, gate results, tool versions, mutation summary when present. The GitHub Action uploads it. The file is not committed.

**FR-5.2** A status check applies only to the sha it ran on. No separate staleness service.

## Non-functional

| ID | Release | Target |
| --- | --- | --- |
| NFR-PERF-01 | v0.1 | `proofsieve guard` path decision p99 under 25 ms, no network, measured on a warm process with a lock of at most 50 globs |
| NFR-PERF-02 | v0.1 | `proofsieve check --gate scope` under 45 s on a 500-line diff, TypeScript or Python |
| NFR-PERF-03 | v0.3 | Incremental mutation under 120 s on a warm CI cache for a diff under 500 lines. Cold PITest is exempt until v0.6 and must print that it skipped the time target |
| NFR-SEC-01 | v0.6 | Reproduction worktrees run in a rootless container, no network, read-only root filesystem, CPU and memory caps. The entrypoint executes a test name, not a shell script |
| NFR-SEC-02 | v0.1 | `guard` and `check --gate scope` send no source to a network API. `test-gen` and review-model calls are off unless `--allow-remote` is set. That flag is absent in v0.1 |
| NFR-REL-01 | v0.1 | `check` exits 0 pass, 1 policy, 2 infra. `guard` exits only 0 or 2. Infra failure in the guard is exit 2 |
| NFR-PORT-01 | v0.1 | Windows amd64 binary. v0.3 adds linux amd64 and darwin arm64. GitHub Action and a GitLab CI template ship with v0.3 |
| NFR-OBS-01 | v0.6 | One JSON log line per gate with `diff_loc`, `files`, `mutation_tier1`, `findings_raw`, `findings_kept`, `risk_score`. OpenTelemetry export is v1.0 |

## CLI

| Command | Release | Behavior |
| --- | --- | --- |
| `proofsieve init` | v0.1 | Write `policy.yaml`, `.gitignore` entry for `.proofsieve/run/`, and hook snippets for Cursor and Claude Code |
| `proofsieve lock --spec <path>` | v0.1 | Write `scope.lock.yaml` |
| `proofsieve guard` | v0.1 | Hook entrypoint |
| `proofsieve check` | v0.1 | Gates enabled by policy. `--format json\|md\|sarif` |
| `proofsieve mutate` | v0.3 | Diff-scoped mutation |
| `proofsieve test-gen` | v0.3 | Clean-room context bundle |
| `proofsieve verify-findings` | v0.6 | Filter a findings JSON file through the reproduction runner |
| `proofsieve attest --pr <n>` | v1.0 | Print the attestation template bound to HEAD. The human pastes it. The bot must not |

## Out of scope

- Replacing Spec Kit, Semgrep, PITest, Stryker, mutmut, or diffguard.
- An IDE plugin with its own editor.
- A hosted multi-tenant scanner. The binary runs in the user's CI.
- Proving full functional correctness. The product proves scope, test sensitivity, and accountable ownership.
