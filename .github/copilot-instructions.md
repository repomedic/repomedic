# Copilot Instructions (RepoMedic)

These instructions are **project-specific**. Follow them when implementing changes in this repository.

`AGENTS.md` is the canonical, repo-wide guide for architecture contracts and extension checklists.
This file is intentionally **Copilot/tool-specific** and should stay short; it points to `AGENTS.md`/`CONTRIBUTING.md` for the full playbooks.

## 1) AI Agent Usability (Critical)

RepoMedic is intended to be **agent-friendly**. Assume the compiled binary is the primary interface.

**Every feature must be discoverable via `--help`:**
- Cobra commands must have meaningful `Short` and **multi-paragraph `Long`** descriptions.
- `Long` must include **Examples** (human + agent-friendly).
- Every flag must have a clear description (what it does, default, constraints).
- If the command emits structured output, document the format and status semantics.

When adding or changing commands/flags, follow this pattern:

```go
var myCmd = &cobra.Command{
    Use:   "mycommand",
    Short: "One-line description",
    Long: `Detailed description.

Examples:
  repomedic mycommand --flag value

  # AI Agent: machine-readable output
  repomedic mycommand --emit ndjson --no-console

Output:
  Describe what is emitted and when.
`,
}
```

## 2) Core Architecture Contracts (Do Not Break)

RepoMedic is **scan-only** and **GitHub-native**.

Non-negotiable contracts (details in `AGENTS.md`):
- **Rules never call GitHub APIs** (rules are pure evaluators over `data.DataContext`).
- **Only the fetch layer calls GitHub** (`internal/fetcher` via `internal/github`).
- **GitHub calls must be bounded** (no unbounded pagination / “fetch everything”).

## 3) Rules: Naming, Files, Registration

**Naming convention (required):**
- Rule implementation filenames must be prefixed with `rule_`.
  - Example: `internal/rules/rule_my_new_rule.go`
- Rule unit test filenames must also be prefixed with `rule_`.
  - Example: `internal/rules/rule_my_new_rule_test.go`

**Rule requirements:**
- `ID()` must be unique and **kebab-case**.
- `Evaluate()` must be deterministic for identical inputs.
- Return statuses consistently: `PASS`, `FAIL`, `ERROR` (see `internal/rules/result.go`).
- Do not mutate GitHub state (RepoMedic is scan-only).

**Registration:**
- Register rules via `init()` in the same file:

```go
func init() {
    Register(&MyNewRule{})
}
```

Avoid duplicate registrations (one rule type registered once).

## 4) Rule Tests (Required for New Rules)

All **new** rules must include a unit test file.

- Use table-driven tests.
- Use `data.NewMapDataContext(...)` to supply dependencies.
- Cover at least: PASS, FAIL, and ERROR (missing dependency / wrong type).
- **Do not mock GitHub API clients** in rule tests.

Minimal skeleton:

```go
func TestMyNewRule_Evaluate(t *testing.T) {
    rule := &MyNewRule{}
    repo := &github.Repository{FullName: github.String("org/repo")}

    tests := []struct {
        name           string
        data           map[data.DependencyKey]any
        expectedStatus Status
    }{ /* ... */ }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            dc := data.NewMapDataContext(tt.data)
            result, err := rule.Evaluate(context.Background(), repo, dc)
            if err != nil { t.Fatalf("Evaluate error: %v", err) }
            if result.Status != tt.expectedStatus {
                t.Fatalf("want %v, got %v", tt.expectedStatus, result.Status)
            }
        })
    }
}
```

## 5) Standard Dev Commands (Prefer Taskfile)

See `CONTRIBUTING.md` for the full dev workflow.

Preferred:
- `task check`
- `task test`
- `task build`

Fallback:
- `go test ./...`
- `go build ./...`

## 6) Change Hygiene

See `AGENTS.md` for detailed hygiene and testing guidance.

- Make small, targeted changes.
- Keep lint/vet/tests clean (treat warnings as build-breaking).
- Keep behavior stable unless the change intends otherwise.
