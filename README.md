# RepoMedic

RepoMedic scans GitHub repositories and organizations to detect risky, missing, or inconsistent configuration.

It helps teams catch security gaps and configuration drift early, locally or in CI.

---

## Install

### Homebrew

```bash
brew install repomedic/tap/repomedic
```

### Winget

```bash
winget install repomedic.repomedic
```

### Go

```bash
go install github.com/repomedic/repomedic@latest
```

---

## Usage

Scan a single repository:

```bash
repomedic scan --repos owner/name
```

Scan an entire GitHub organization:

```bash
repomedic scan --org my-org
```

Authenticate using the GitHub CLI (preferred):

```bash
gh auth login --scopes "repo admin:org"
```

Or using an environment variable:

```bash
export GITHUB_TOKEN=ghp_...
```

---

## Example output

```text
[PASS] repo-a: default-branch-protected
[FAIL] repo-a: codeowners-exists - CODEOWNERS file is missing
[PASS] repo-b: branch-protect-enforce-admins
```

Exit codes:
- 0: all checks passed
- 1: one or more checks failed
- 2: partial failure (error evaluating some rules)
- 3: fatal error


---

## Rules

RepoMedic audits configuration state using deterministic rules:

- **Branch Protection**: Checks for PR reviews, status checks, push restrictions, and admin enforcement.
- **Secret Scanning**: Verifies enablement and availability.
- **Repository Standards**: Enforces descriptions, READMEs, and visibility settings.
- **CODEOWNERS**: Checks for the existence of ownership definitions.
- **Merge Method Consistency**: Ensures repositories follow a consistent merge policy.


---

## License

Apache 2.0
