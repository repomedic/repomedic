package config

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	// MAINTAINER NOTE: If you add/change/remove config fields that affect scan
	// behavior, keep these in sync:
	// - CLI flags in internal/cli/scan.go
	// - report reproducibility command in internal/engine/engine.go:buildReproducibilityCommand
	Targeting Targeting
	Rules     Rules
	Output    Output
	Runtime   Runtime
}

type Targeting struct {
	// Org is the GitHub organization account to scan (name or URL; see --org).
	Org string

	// User is the GitHub user account to scan (name or URL; see --user).
	User string

	// Enterprise is the GitHub enterprise slug to scan (name or URL; see --enterprise).
	// Note: enterprise scanning is not yet implemented.
	Enterprise string

	// Repos is an explicit list of repositories to scan as OWNER/REPO (see --repos).
	// Values may be provided as repeated flags and/or comma-separated lists.
	Repos []string

	// Include filters repositories by name using Go path.Match style (see --include).
	// If a pattern contains '/', it matches OWNER/REPO; otherwise it matches repo name.
	Include []string

	// Exclude filters repositories by name using Go path.Match style (see --exclude).
	// Same matching rules as Include.
	Exclude []string

	// Topic requires repositories to have at least one matching topic (exact match; see --topic).
	// Values may be provided as repeated flags and/or comma-separated lists.
	Topic []string

	// Visibility filters repositories by visibility (see --visibility).
	// Allowed values: public, private, internal, all.
	Visibility string

	// Archived controls how archived repos are handled (see --archived).
	// Allowed values: include, exclude, only.
	Archived string

	// Forks controls how forked repos are handled (see --forks).
	// Allowed values: include, exclude, only.
	// Note: if --user is set and --forks is omitted, forks default to include.
	Forks string

	// MaxRepos limits how many repositories to scan (see --max-repos). 0 means unlimited.
	MaxRepos int

	// DryRun resolves the repo set and prints the scan plan without scanning (see --dry-run).
	DryRun bool
}

type Rules struct {
	// Selector selects which rules to run.
	// Empty means all rules; otherwise it is a rule selector expression (see --rules).
	Selector string

	// Set provides per-rule option overrides from the CLI.
	// Entries are of the form ruleID.option=value (repeatable; comma-separated accepted; see --set).
	Set []string

	// Evidence controls how much supporting detail rules include in results (see --evidence).
	// Allowed values: minimal, standard, full.
	Evidence string
}

type Output struct {
	// ConsoleFormat controls the human-facing console sink format (see --console-format).
	// Allowed values: text, json, ndjson.
	ConsoleFormat string

	// ConsoleFilterStatus filters console output by result status (see --console-filter-status).
	// Allowed values: PASS, FAIL, ERROR.
	ConsoleFilterStatus []string

	// Report writes a Markdown report to this path (see --report).
	Report string

	// Out writes structured output to this path (see --out).
	Out string

	// OutFormat selects the format for --out (see --out-format).
	// Allowed values: json, ndjson. If empty, it is inferred from the --out file extension.
	OutFormat string

	// Emit writes an additional structured event stream to stdout (see --emit).
	// Allowed values: json, ndjson.
	Emit []string

	// NoConsole suppresses the console sink (see --no-console).
	// Use with --emit/--out/--report for machine-readable output.
	NoConsole bool
}

type Runtime struct {
	// Concurrency controls parallelism for repository processing (see --concurrency).
	// Must be >= 1.
	Concurrency int

	// Timeout is the global scan timeout for the run (see --timeout).
	// Must be > 0.
	Timeout time.Duration

	// FailFast stops the scan on the first fatal error (see --fail-fast).
	FailFast bool

	// Verbose enables more detailed diagnostics (primarily for dependency/fetch failures).
	Verbose bool
}

func New() *Config {
	return &Config{
		Targeting: Targeting{
			Visibility: "all",
			Archived:   "exclude",
			Forks:      "exclude",
		},
		Rules: Rules{
			Evidence: "standard",
		},
		Output: Output{
			ConsoleFormat: "text",
		},
		Runtime: Runtime{
			Concurrency: 5,
			Timeout:     30 * time.Minute,
		},
	}
}

func (c *Config) Validate() error {
	// Normalize comma-delimited list inputs.
	c.Targeting.Repos = splitCommaList(c.Targeting.Repos)
	c.Targeting.Topic = splitCommaList(c.Targeting.Topic)
	c.Rules.Set = splitCommaList(c.Rules.Set)

	// Normalize account selectors.
	if c.Targeting.Org != "" {
		org, err := normalizeAccountSelector(c.Targeting.Org)
		if err != nil {
			return fmt.Errorf("invalid --org value: %w", err)
		}
		c.Targeting.Org = org
	}
	if c.Targeting.User != "" {
		user, err := normalizeAccountSelector(c.Targeting.User)
		if err != nil {
			return fmt.Errorf("invalid --user value: %w", err)
		}
		c.Targeting.User = user
	}
	if c.Targeting.Enterprise != "" {
		ent, err := normalizeEnterpriseSelector(c.Targeting.Enterprise)
		if err != nil {
			return fmt.Errorf("invalid --enterprise value: %w", err)
		}
		c.Targeting.Enterprise = ent
	}

	// Targeting validation
	if c.Targeting.Org == "" && c.Targeting.User == "" && c.Targeting.Enterprise == "" && len(c.Targeting.Repos) == 0 {
		return errors.New("at least one of --org, --user, --enterprise, or --repos must be provided")
	}
	if c.Targeting.Org != "" && c.Targeting.User != "" {
		return errors.New("--org and --user are mutually exclusive")
	}

	// Output validation
	c.Output.ConsoleFormat = normalizeEnumValue(c.Output.ConsoleFormat)
	if c.Output.ConsoleFormat == "" {
		return errors.New("--console-format must be one of: text, json, ndjson")
	}
	if c.Output.ConsoleFormat != "text" && c.Output.ConsoleFormat != "json" && c.Output.ConsoleFormat != "ndjson" {
		return fmt.Errorf("unsupported --console-format: %s (must be one of: text, json, ndjson)", c.Output.ConsoleFormat)
	}

	for _, emit := range c.Output.Emit {
		v := normalizeEnumValue(emit)
		if v == "" {
			return errors.New("--emit must be one of: json, ndjson")
		}
		if v != "json" && v != "ndjson" {
			return fmt.Errorf("unsupported --emit value: %s (must be one of: json, ndjson)", v)
		}
	}

	// Rules validation
	c.Rules.Evidence = normalizeEnumValue(c.Rules.Evidence)
	if c.Rules.Evidence == "" {
		return errors.New("--evidence must be one of: minimal, standard, full")
	}
	if c.Rules.Evidence != "minimal" && c.Rules.Evidence != "standard" && c.Rules.Evidence != "full" {
		return fmt.Errorf("unsupported --evidence: %s (must be one of: minimal, standard, full)", c.Rules.Evidence)
	}

	// Targeting enum validation
	c.Targeting.Visibility = normalizeEnumValue(c.Targeting.Visibility)
	if c.Targeting.Visibility == "" {
		c.Targeting.Visibility = "all"
	}
	if c.Targeting.Visibility != "public" && c.Targeting.Visibility != "private" && c.Targeting.Visibility != "internal" && c.Targeting.Visibility != "all" {
		return fmt.Errorf("unsupported --visibility: %s (must be one of: public, private, internal, all)", c.Targeting.Visibility)
	}

	c.Targeting.Archived = normalizeEnumValue(c.Targeting.Archived)
	if c.Targeting.Archived == "" {
		c.Targeting.Archived = "exclude"
	}
	if c.Targeting.Archived != "include" && c.Targeting.Archived != "exclude" && c.Targeting.Archived != "only" {
		return fmt.Errorf("unsupported --archived: %s (must be one of: include, exclude, only)", c.Targeting.Archived)
	}

	c.Targeting.Forks = normalizeEnumValue(c.Targeting.Forks)
	if c.Targeting.Forks == "" {
		c.Targeting.Forks = "exclude"
	}
	if c.Targeting.Forks != "include" && c.Targeting.Forks != "exclude" && c.Targeting.Forks != "only" {
		return fmt.Errorf("unsupported --forks: %s (must be one of: include, exclude, only)", c.Targeting.Forks)
	}

	// Runtime validation
	if c.Targeting.MaxRepos < 0 {
		return errors.New("--max-repos must be >= 0")
	}
	if c.Runtime.Concurrency <= 0 {
		return errors.New("--concurrency must be >= 1")
	}
	if c.Runtime.Timeout <= 0 {
		return errors.New("--timeout must be > 0")
	}

	if c.Output.Out != "" {
		c.Output.OutFormat = normalizeEnumValue(c.Output.OutFormat)
		if c.Output.OutFormat == "" {
			ext := strings.ToLower(filepath.Ext(c.Output.Out))
			switch ext {
			case ".json":
				c.Output.OutFormat = "json"
			case ".ndjson":
				c.Output.OutFormat = "ndjson"
			default:
				if ext == "" {
					return errors.New("cannot infer output format from file extension (missing extension); use --out-format")
				}
				return fmt.Errorf("cannot infer output format from file extension %q; use --out-format", ext)
			}
		} else {
			if c.Output.OutFormat != "json" && c.Output.OutFormat != "ndjson" {
				return fmt.Errorf("unsupported output format: %s", c.Output.OutFormat)
			}
		}
	}

	// Ruleset option syntax validation (rule.option=value)
	if len(c.Rules.Set) > 0 {
		if _, err := ParseRuleOptionAssignments(c.Rules.Set); err != nil {
			return err
		}
	}

	return nil
}

func normalizeEnumValue(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func normalizeEnterpriseSelector(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}

	// Accept a raw enterprise slug, or a GitHub URL like:
	//   https://github.com/enterprises/<slug>
	//   github.com/enterprises/<slug>
	if strings.HasPrefix(raw, "github.com/") || strings.HasPrefix(raw, "www.github.com/") {
		raw = "https://" + raw
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		u, err := url.Parse(raw)
		if err != nil {
			return "", fmt.Errorf("%q", raw)
		}
		host := strings.ToLower(u.Hostname())
		if host == "www.github.com" {
			host = "github.com"
		}
		if host != "github.com" {
			return "", fmt.Errorf("%q", raw)
		}
		parts := strings.FieldsFunc(strings.Trim(u.Path, "/"), func(r rune) bool { return r == '/' })
		if len(parts) < 2 || parts[0] != "enterprises" {
			return "", fmt.Errorf("%q", raw)
		}
		return parts[1], nil
	}

	// Basic sanity: reject obvious owner/repo-like inputs.
	if strings.Contains(raw, "/") {
		return "", fmt.Errorf("%q", raw)
	}
	return raw, nil
}

func normalizeAccountSelector(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}

	// Accept a raw account name, or a GitHub URL like:
	//   https://github.com/<name>
	//   https://github.com/orgs/<name>
	//   https://github.com/users/<name>
	//   github.com/<name>
	if strings.HasPrefix(raw, "github.com/") || strings.HasPrefix(raw, "www.github.com/") {
		raw = "https://" + raw
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		u, err := url.Parse(raw)
		if err != nil {
			return "", fmt.Errorf("%q", raw)
		}
		host := strings.ToLower(u.Hostname())
		if host == "www.github.com" {
			host = "github.com"
		}
		if host != "github.com" {
			return "", fmt.Errorf("%q", raw)
		}
		parts := strings.FieldsFunc(strings.Trim(u.Path, "/"), func(r rune) bool { return r == '/' })
		if len(parts) == 0 {
			return "", fmt.Errorf("%q", raw)
		}
		if parts[0] == "orgs" || parts[0] == "users" {
			if len(parts) < 2 {
				return "", fmt.Errorf("%q", raw)
			}
			return parts[1], nil
		}
		return parts[0], nil
	}

	// Basic sanity: reject obvious repo-like inputs.
	if strings.Contains(raw, "/") {
		return "", fmt.Errorf("%q", raw)
	}
	return raw, nil
}

// ParseRuleOptionAssignments parses values of the form "ruleID.option=value".
//
// Notes:
// - Entries may be provided via repeated flags and/or comma-delimited lists.
// - This validates syntax only (no validation of rule IDs or option names).
// - Empty values are allowed ("rule.option=").
func ParseRuleOptionAssignments(values []string) (map[string]map[string]string, error) {
	out := make(map[string]map[string]string)
	for _, raw := range splitCommaList(values) {
		left, value, ok := strings.Cut(raw, "=")
		if !ok {
			return nil, fmt.Errorf("invalid --set entry %q: expected rule.option=value", raw)
		}
		left = strings.TrimSpace(left)
		value = strings.TrimSpace(value)
		ruleAndOpt := strings.TrimSpace(left)
		ruleID, opt, ok := strings.Cut(ruleAndOpt, ".")
		if !ok {
			return nil, fmt.Errorf("invalid --set entry %q: expected rule.option=value", raw)
		}
		ruleID = strings.TrimSpace(ruleID)
		opt = strings.TrimSpace(opt)
		if ruleID == "" || opt == "" {
			return nil, fmt.Errorf("invalid --set entry %q: expected non-empty rule and option", raw)
		}
		if _, ok := out[ruleID]; !ok {
			out[ruleID] = make(map[string]string)
		}
		out[ruleID][opt] = value
	}
	return out, nil
}

func splitCommaList(values []string) []string {
	var out []string
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			p := strings.TrimSpace(part)
			if p == "" {
				continue
			}
			out = append(out, p)
		}
	}
	return out
}
