# Contributing to RepoMedic

## Start Here (Agents + Humans)

The canonical, agent-focused guide for RepoMedic is `AGENTS.md`.

If you're adding rules or fetchers, read `AGENTS.md` first for the architecture contracts (rules never call GitHub, bounded pagination, dependency semantics) and the extension checklists.

## Adding a New Rule

Rules are the core of RepoMedic. To add a new rule:

1.  **Create a new file** in `internal/rules/`.
    -   Rule implementation files must be prefixed with `rule_` (e.g., `rule_my_new_rule.go`).
    -   Rule unit test files must also be prefixed with `rule_` (e.g., `rule_my_new_rule_test.go`).
2.  **Define the Rule Struct**:
    ```go
    type MyNewRule struct{}
    ```
3.  **Implement the `Rule` Interface**:
    -   `ID()`: Unique identifier (kebab-case).
    -   `Title()`: Human-readable title.
    -   `Description()`: What the rule checks.
    -   `Dependencies()`: Return the list of `DependencyKey`s required.
    -   `Evaluate()`: Implement the logic using `DataContext`.
4.  **Register the Rule**:
    Add an `init()` function to register the rule.
    ```go
    func init() {
        Register(&MyNewRule{})
    }
    ```
5.  **Add Unit Tests**:
    Create `internal/rules/rule_my_new_rule_test.go`.
    -   Use `data.NewMapDataContext` to mock data.
    -   Test PASS, FAIL, and ERROR scenarios.
    -   **Do not mock GitHub API** in rule tests.

## Development Workflow

RepoMedic uses a `Taskfile.yml` to make local development and CI checks consistent.

1.  **Install dev tools**: `task tools:install`
2.  **Run the same checks as CI**: `task check`
    - Runs: gofmt verify, go vet, golangci-lint, govulncheck, and go test.
3.  **Build**: `task build`
4.  **Run from source**: `task run -- <args>`

If you don't have Task installed, you can still run the underlying commands directly:
- `gofmt -l .`
- `go vet ./...`
- `go test ./...`

## Code Style

-   Follow standard Go conventions.
-   Keep rules isolated and stateless.
-   Use `internal/data/keys.go` for dependency keys.

## Adding a New Dependency (DataFetcher)

If a rule needs data that RepoMedic does not fetch yet:

1. Add a new `DependencyKey` constant in `internal/data/keys.go`.
2. Implement the GitHub API fetch in `internal/fetcher` as a new `fetch_*.go` file.
    - Each dependency key should have its own file (keep implementations small and focused).
    - Register the fetch implementation via `init()` (duplicate registrations panic).
    - Acquire request budget before each GitHub API call and update `RequestBudget` from response headers.
    - Keep pagination bounded (never “fetch everything”).

### Fetcher composition (dependencies between fetchers)

Fetchers may call other fetchers via `Fetcher.Fetch(...)` to avoid duplicated GitHub API calls.

- Prefer composition when you would otherwise call the same endpoint twice.
- Cycles are treated as errors and must fail fast (e.g., A → B → A).

### Testing expectations

Add tests when the fetcher has non-trivial behavior (branching, pagination, special-case error handling, or composition):

- Use `net/http/httptest` to mock GitHub endpoints.
- Prefer table-driven tests with `t.Run(...)`.
- Assert behaviors (returned types, pagination bounds, call counts when part of the contract) rather than internal implementation details.
