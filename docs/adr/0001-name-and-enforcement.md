# ADR 0001 — Name and enforcement point

- Status: accepted
- Date: 2026-11-21

## Name

The project is **ProofSieve**. The binary is `proofsieve`. Repository policy and per-change locks live in `.proofsieve/`.

The name says what the tool does: sieve a large agent diff, a noisy review, and a hollow test suite down to something proven. A web search on 2026-11-21 found nearby projects (ProofGate, Proofwright, Proofrail, RunSieve) and no shipping project under this name.

Config paths from earlier drafts (`.tenet/`, the `tenet` binary) are retired.

## Enforcement point

The local control is a **hook-invoked guard**, not an MCP filesystem proxy.

Cursor and Claude Code apply native `Write`, `Edit`, and `Delete` tools inside the agent. Those calls do not pass through a user-supplied MCP server, so a proxy only blocks agents that were convinced to use the proxy's own tools. Rules files are hints. The model can ignore them.

`proofsieve guard` reads the hook payload on stdin and decides allow or deny before the tool runs:

- Cursor: `.cursor/hooks.json` event `preToolUse`, matcher `Write|Delete|Shell`, plus `beforeShellExecution`.
- Claude Code: `PreToolUse` matcher `Edit|Write|Bash`. Cursor loads this format when third-party hooks are enabled.

Hook contract (Cursor and Claude Code):

| Guard exit | Meaning |
| --- | --- |
| 0 | Allow. JSON `permission: allow` on stdout. |
| 2 | Deny. The tool does not run. Stdout or stderr explains the violation so the agent can stay in scope. |

Any other exit code **fail-opens** in both products. On an internal error the guard exits `2`. A broken guard must not become a silent allow.

CI remains the authority. Shell redirection, `git apply`, and a disabled hook can still write files. `proofsieve check` on the pull request is a required status check. The hook exists so the agent corrects itself in the same turn. The check exists so a bypass cannot merge.

## Out of scope for this decision

- Embedding Open Policy Agent in the hook path. v0.1 evaluates YAML in-process. Rego is a v1.0 extension point.
- Treating an MCP review bot as a merge gate.
- Generating tests with a model and calling that output the oracle.
