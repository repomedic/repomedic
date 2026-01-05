# RepoMedic: Agent Guide (AGENTS.md)

This file is the canonical, agent-focused guide for making safe, consistent changes to RepoMedic.

If you are a human contributor, this is still useful as a “how to extend RepoMedic without breaking architecture contracts” checklist.

Related docs:
- ARCHITECTURE.md: system design + component boundaries
- CONTRIBUTING.md: contributor workflow + dev commands

## Core Contracts (Do Not Break)

RepoMedic is **scan-only** and **GitHub-native**.

- **Rules never call GitHub APIs.**
  - Rules live in `internal/rules`.
  - Rules are pure evaluators over a `data.DataContext`.
  - The only code allowed to call GitHub is the fetch layer (`internal/fetcher` via `internal/github`).

- **Dependencies are explicit.**
  - Rules declare data needs via `Dependencies()` returning `[]data.DependencyKey`.
  - The engine plans/deduplicates/schedules fetches, then evaluates rules.

- **Keep GitHub calls bounded.**
  - Do not introduce unbounded pagination or “fetch everything.”
  - Prefer bounded pagination with clear safety caps.
  - Always acquire/update the request budget around API calls.

- **Determinism.**
  - Given identical inputs (`repo` + `DataContext` values), rule results must be deterministic.

## CLI Discoverability (Agent-Friendly UX)

Assume the compiled binary is the primary interface.

- Every feature must be discoverable via `--help`.
- Cobra commands should have a meaningful `Short` and a multi-paragraph `Long` that includes:
  - examples (human-friendly)
  - examples that are machine-friendly (e.g., `--emit ndjson --no-console`)
  - output format notes if structured output is emitted

## Status & Error Semantics (Practical Guidance)

RepoMedic rules return a `Result` with a status. Use consistent semantics:

- **PASS**: Requirement is satisfied.
- **FAIL**: Requirement is not satisfied.
- **ERROR**: The rule could not be evaluated correctly (missing dependency, wrong type, etc.).

Dependency values commonly have three states:

- **Missing key** in `DataContext`: treated as **ERROR** (programmer/config bug).
- Key present with **nil value**: usually means “known absence” (e.g., 404 → not configured) and should typically drive **FAIL** or a specific rule interpretation.
- Key present with a non-nil value of the **wrong type**: **ERROR**.

Note: Some dependency fetch failures may be surfaced as SKIP by the engine (e.g., forbidden on specific governance endpoints), depending on the engine’s presentation rules.

## How to Add a Rule (Checklist)

1. Create a new file in `internal/rules/`:
   - Implementation: `internal/rules/rule_<name>.go`
   - Tests: `internal/rules/rule_<name>_test.go`
2. Implement the Rule interface:
   - `ID()` must be unique and kebab-case.
   - `Dependencies()` declares required dependency keys.
   - `Evaluate()` reads from `DataContext` only.
3. Register the rule once via `init()` in the same file:
   - `Register(&MyRule{})`
4. Add table-driven unit tests:
   - Use `data.NewMapDataContext(...)`.
   - Cover at least: PASS, FAIL, ERROR (missing dep and/or wrong type).
   - Do not mock GitHub API clients in rule tests.

## How to Add New Data (Dependency + Fetcher)

If a rule needs new data that isn’t fetched yet:

1. Add a new dependency key in `internal/data/keys.go`.
2. Implement a fetcher in `internal/fetcher/fetch_*.go`:
   - One key per file.
   - Register via `init()`.
   - Acquire request budget before each GitHub call and update from response.
   - Keep pagination bounded.
3. Consume the data in rules via `DataContext`.

## Comment Hygiene (Dependency Keys & Fetchers)

Keep comments stable and semantic so they don’t churn when implementation details change.

- Dependency key comments in `internal/data/keys.go` should describe *what the dependency represents* (the meaning of the value put into `DataContext`), not which API/endpoint is called.
- Avoid coupling dependency comments to any specific rule (fetchers/dependencies may be used by many rules).
- Prefer describing shape/semantics (e.g., “presence of X”, “effective rules for default branch”, “resolved path for Y”) and any important invariants (e.g., “bounded”, “default branch”).
- If implementation details matter, capture them in the fetcher implementation (and tests), not in the dependency key comment.

Fetcher composition is allowed: a fetcher may call `Fetcher.Fetch(...)` for other keys to avoid duplicate API calls. Cycles must fail fast.

## Standard Dev Commands

Prefer Task (most consistent with CI):

- `task check` (format verify, vet, lint if installed, govulncheck, tests)
- `task test`
- `task build`
- `task run -- <args>`

Fallback:
- `go test ./...`
- `go build ./...`

## Change Hygiene

- Make small, targeted changes.
- Avoid adding dependencies unless necessary.
- Do not introduce new lint/vet/test warnings.
- Do not mutate GitHub state (RepoMedic is scan-only).
