# RepoMedic

### The Governance Auditor for GitHub Fleets

**RepoMedic is a single-binary, read-only auditor that scans your entire GitHub organization for governance drift and security-relevant misconfigurations.**

It answers one uncomfortable question:

> **“Are our repositories actually protected; or do we just assume they are?”**

RepoMedic runs locally, never mutates state, and produces a clear, deterministic audit you can trust.

---

## GitHub Entropy Is Inevitable

As organizations scale, GitHub configuration quietly decays.

* Branch protections get relaxed “just for this hotfix”
* Review rules drift over time
* New repos are created without guardrails
* Security features are assumed to be “on”

Multiply this across **hundreds of repositories**, and no one has a complete picture anymore.

RepoMedic performs a **cold, factual audit** of your GitHub fleet and reports the **actual state**, not the intended one.

No dashboards.
No agents.
No enforcement.

Just evidence.

---

## The 30-Second Audit

If you have a GitHub token, you can audit your organization **right now**.

```bash
repomedic scan --org your-company --report audit.md
```

In seconds, RepoMedic generates a **Markdown audit report** highlighting:

* Repositories with **no branch protection** on production
* Repos where **admins can bypass PR requirements**
* **Force-push enabled** on default branches
* Missing **CODEOWNERS** enforcement
* Rulesets in **evaluate** or **disabled** mode

This is the report you wish you had *before* the incident, not after.

---

## Why Engineering Leaders Use RepoMedic

### 🛡️ Zero-Risk by Design

RepoMedic is **read-only**.
It cannot break builds, delete branches, or modify settings.

You can safely run it against production organizations, in CI, or during audits - without approvals or rollback plans.

---

### ⚙️ Fleet-Scale, Not Repo-By-Repo

RepoMedic is built for **large GitHub estates**.

* Bounded, concurrent API usage
* Rate-limit aware
* Predictable execution

Whether you have 20 repos or 500, RepoMedic gives you a single, coherent answer.

---

### 🧠 Deterministic & CI-Safe

Same inputs → same outputs.

* Stable exit codes
* No hidden heuristics
* No “best-guess” scoring

RepoMedic fits cleanly into CI/CD pipelines and compliance workflows.

---

### 🤖 Agent & Automation Native

RepoMedic is designed to be **machine-consumable**.

* Stream lifecycle events with `--emit ndjson`
* Feed LLMs, SIEMs, or internal automation
* Treat governance drift as structured data, not screenshots

---

### 🔍 Governance, Not Code Linting

RepoMedic doesn’t lint code.

It audits **GitHub itself**:

* Branch protection rules
* Review enforcement
* CODEOWNERS behavior
* Repository visibility and hygiene
* Security feature enablement

This is governance where it actually lives.

---

## What RepoMedic Is *Not*

To be explicit:

* ❌ Not a fixer
* ❌ Not an enforcer
* ❌ Not a SaaS
* ❌ Not a GitHub App
* ❌ Not another dashboard

RepoMedic finds **what’s wrong**, safely and objectively - and lets you decide what to do next.

---

## High-Signal Checks (The “Oh Sh*t” List)

RepoMedic ships with governance rules designed to catch issues that fail audits and trigger incidents:

| Category              | What RepoMedic Detects                         | Why It Matters                                   |
| --------------------- | ---------------------------------------------- | ------------------------------------------------ |
| **Branch Protection** | PRs not required, force-push enabled, branch deletion allowed | Prevents history rewriting and unreviewed merges |
| **Review Hygiene**    | Stale approvals, missing CODEOWNER enforcement | Stops unseen code from reaching production       |
| **Admin Bypass**      | Admins allowed to ignore protections           | Eliminates silent privilege escalation           |
| **Repo Hygiene**      | Missing README, CODEOWNERS, or description     | Signals unmanaged or orphaned repos              |
| **Ruleset Health**    | Rulesets in "evaluate" or "disabled" mode      | Ensures governance is actually enforced          |

All checks are **objective, auditable, and deterministic**.

---

## Local Execution. Zero Exfiltration.

RepoMedic runs entirely on your machine or CI runner.

* No SaaS
* No background agents
* No webhooks
* No data leaving your infrastructure

Your audit data stays where it belongs.

---

## Installation

### Recommended: Single Binary

Download the appropriate binary for macOS, Linux, or Windows from the Releases page.

No dependencies. No setup.

### Go Install

```bash
go install github.com/your-org/repomedic/cmd/repomedic@latest
```

### Authentication

RepoMedic reads credentials from your environment:

```bash
export GITHUB_TOKEN=...
```

If the GitHub CLI (`gh`) is installed, RepoMedic can automatically reuse your authenticated session.

---

## Typical Workflows

**The Monday Morning Audit**

```bash
repomedic scan --org acme-corp --report weekly-audit.md
```

**The CI Gatekeeper**

```bash
repomedic scan --repos acme-corp/billing-service --no-console
# Exit code 1 = governance drift detected
```

**The Automation Feed**

```bash
repomedic scan --org acme-corp --emit ndjson
```

---

## RepoMedic in One Sentence

> **RepoMedic gives engineering leaders a truthful, read-only answer to the question: “Is our GitHub actually governed; or just assumed to be?”**

---

## License

Apache 2.0 - Free to use for internal auditing.
