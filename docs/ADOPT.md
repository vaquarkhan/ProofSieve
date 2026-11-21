# Adopt, study, or leave

Checked 2026-11-21 against project docs and source. No single tool covers pre-write denial, locked oracle tests, proof-only review comments, and a human attestation. ProofSieve orchestrates the mature pieces and builds the three gaps that are still open.

## Decision summary

| Need | Existing tool | Decision |
| --- | --- | --- |
| Spec before code | [github/spec-kit](https://github.com/github/spec-kit) | **Adopt.** `constitution.md`, `spec.md`, `plan.md`, `tasks.md`. ProofSieve stores `spec_ref` and never rewrites the spec flow |
| Spec on an existing codebase | [Fission-AI/OpenSpec](https://github.com/Fission-AI/OpenSpec) | **Adopt as the alternative.** Change folders with proposal, spec deltas, tasks. Pick one spec tool per repo |
| EARS requirements inside an IDE | Kiro (AWS) | **Leave.** Useful authoring UX. Depending on it would tie the gate to one IDE. Spec Kit files are the interchange |
| Spec plugin and skills | Tessl | **Leave.** Overlaps Spec Kit for our purposes |
| Post-diff path, file, and line budgets | [JohnODowdAI/scopefence](https://github.com/JohnODowdAI/scopefence) | **Study, reimplement.** Policy shape (include, forbid, budgets, added-line patterns) is right. It checks git after the write, is Python, and is still a beta. The hook needs a static binary and a deny before the write |
| Agent write scope | [floomhq/agent-scope](https://github.com/floomhq/agent-scope) | **Study.** Same post-hoc idea. Confirms the policy shape |
| Cognitive complexity, size, tiered mutation on the diff | [0xPolygon/diffguard](https://github.com/0xPolygon/diffguard) | **Adopt for Go, Rust, and TypeScript mutation.** Caps match ours (cognitive complexity 10, 50-line functions). Tier-1 logic mutants gate; observability mutants do not. MIT, Go. Do not reimplement those operators |
| Java / .NET / Python mutation | PITest, Stryker, mutmut | **Adopt behind one adapter interface.** diffguard does not cover Java or Python |
| Deterministic SAST | [Semgrep](https://semgrep.dev/) CLI `--sarif` | **Adopt local CLI.** The hosted Semgrep MCP is optional and off by default because it leaves the machine |
| Architecture fitness | dependency-cruiser (TS), ArchUnit (Java) | **Adopt in v0.6** for the languages in the repo |
| Empty tests, stubbed security checks, missing imports | [aevryone/proofgate](https://github.com/aevryone/proofgate) | **Adopt the detectors, not the app.** Blockers must be structural. Their lesson: do not fail a merge on a guess about English. Their scope detector is a warning; ours blocks, because we have an explicit lock |
| Claim did not survive the parent commit | [bogacsmz/proofwright](https://github.com/bogacsmz/proofwright) | **Adopt the verdict rule.** A bug claim is kept only when the cited test passes on base and fails on head. No evidence is `dropped`, never `pass` |
| Deterministic multi-scanner review | [gitrdunhq/caliper](https://github.com/gitrdunhq/caliper) | **Study for v1.0.** Useful prior art for SARIF fan-in. Too large to embed |
| PR file-size comments | Danger, CodeAnt | **Leave.** `proofsieve check` is the required status. A second comment bot recreates the noise problem |
| LLM review with repo memory | CodeRabbit, Greptile | **Leave as judges.** Call-graph context is a good idea for the behavioral diff. Their comment stream is the failure mode in the problem statement |
| Pre-write denial | Cursor `preToolUse`, Claude Code `PreToolUse` | **Adopt as the local plane.** Documented contract: exit 2 denies; any other non-zero exit fail-opens |

## What is actually missing

These four behaviors were not found as one product, and they are the build list:

1. **Pre-write deny** tied to a spec lock, including a deny of the lock file itself, with exit code 2 so the agent sees the violation in the same turn.
2. **Oracle checksums** so the agent cannot weaken the tests that will grade the patch. ScopeFence forbids paths. It does not hash assertion bodies.
3. **A review publisher that defaults to silence** and posts a model finding only after a base/head reproduction. Proofwright verifies claims the author already made. It does not cap an unsolicited review bot.
4. **An attestation the machine can check** (sha, spec id, real test name) without asking another model whether the author "understands" the diff.

## Explicit non-goals from the survey

- Wrapping Semgrep, PITest, or diffguard in a new mutation engine.
- Putting Open Policy Agent on the 25 ms hook path. Caliper shows Rego is valuable at CI time. v1.0 can shell out to `opa eval`. v0.1 evaluates a closed YAML schema in Go.
- A clean-room LLM test generator as a merge signal. Kiro can emit property tests from a spec. Those tests are drafts until a human locks the digest and Tier-1 mutants die.

## Language order

v0.1 parses TypeScript and Python, because that is where unconstrained agent edits show up most often in application repos. Go is the implementation language of the binary and is a cheap third grammar. Java waits for the PITest adapter in v0.6. This splits from a "all languages in the first binary" plan on purpose: an unfinished tree-sitter grammar that false-passes is worse than a documented unsupported language, which exits `2`.
