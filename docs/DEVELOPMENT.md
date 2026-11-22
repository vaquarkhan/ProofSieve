# Development

ProofSieve is a Go 1.23 module, `github.com/vaquarkhan/ProofSieve`. Production code lives under `cmd/proofsieve` and `internal/`. The site in `site/` is static HTML published by GitHub Pages.

## Build and test

```
go test ./... -count=1 -coverprofile=cover.out -covermode=atomic
go tool cover -func=cover.out
gofmt -w cmd internal
```

CI fails unless the coverage total is `100.0%`. On Windows, tests that create git repositories set `user.email`, `user.name`, `commit.gpgsign=false`, and `core.autocrlf=false` inside the temporary repository only. Do not change the global git config.

The local toolchain used while developing this tree was Go 1.23. First compiles on a 32-bit toolchain are slow. A full `go test ./...` can take a few minutes because several packages create real git worktrees.

## Layout

| Path | Responsibility |
| --- | --- |
| `internal/check` | Load policy and lock, diff base...head, run every requested gate |
| `internal/behavior` | Signature changes, sensitive calls, callers, quiz facts |
| `internal/guard` | stdin hook JSON, session file at `.proofsieve/run/session.json` |
| `internal/lang` | Lexers and cognitive complexity for Python, TypeScript, JavaScript, and Go |
| `internal/mutate` | Mutants applied in a temp copy of the tree |
| `internal/review` | SARIF ingest and worktree reproduction |
| `internal/cli` | Commands and process exit codes |

`cmd/proofsieve` sets `exit` to `os.Exit` so tests can call `main` without leaving the process.

## Adding a language

1. Map the file extension in `lang.Language`.
2. Teach `Tokens` and `Analyze` the grammar you need for complexity. Unsupported syntax must return an error.
3. Return true from `Supported` only after both exist. Recognized but unsupported languages fail the complexity gate with rule `unparsed`.
4. Add mutation operators only for tokens you can restore exactly. Skip test paths.
5. Cover the new branches. The CI total must stay `100.0%`.

## Exit codes

| Code | check | guard |
| --- | --- | --- |
| 0 | All requested gates passed | Allow the tool call |
| 1 | A gate reported an error finding | unused |
| 2 | Usage, git, policy, lock, or a command could not run | Deny the tool call |

Guard infrastructure errors exit 2 on purpose. A broken hook must not fail open.

## Threat model

The editor hook is best-effort. CI is authoritative. Commands are argv vectors, not shell strings. Mutation does not edit the user worktree. Review test ids reject newlines and shell metacharacters. `allow_remote_model: true` and `tier3_gates: true` are rejected at policy load. Attestation authors whose login contains `[bot]` are rejected. Writes under `.proofsieve` are denied by the hook even when an allow glob matches.
