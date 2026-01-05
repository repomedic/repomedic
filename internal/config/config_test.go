package config

import (
	"reflect"
	"testing"
)

func TestValidate_NormalizesCommaDelimitedRepos(t *testing.T) {
	cfg := New()
	cfg.Targeting.Repos = []string{"acme/foo, acme/bar", "acme/baz", ",,"}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	want := []string{"acme/foo", "acme/bar", "acme/baz"}
	if !reflect.DeepEqual(cfg.Targeting.Repos, want) {
		t.Fatalf("Repos normalized mismatch: got %v want %v", cfg.Targeting.Repos, want)
	}
}

func TestValidate_NormalizesCommaDelimitedTopics(t *testing.T) {
	cfg := New()
	cfg.Targeting.Repos = []string{"acme/repo"}
	cfg.Targeting.Topic = []string{"security, compliance", "devops", ",,"}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	want := []string{"security", "compliance", "devops"}
	if !reflect.DeepEqual(cfg.Targeting.Topic, want) {
		t.Fatalf("Topic normalized mismatch: got %v want %v", cfg.Targeting.Topic, want)
	}
}

func TestParseRuleOptionAssignments(t *testing.T) {
	got, err := ParseRuleOptionAssignments([]string{
		"branch-protection.required_reviews=2, pr-reviews.min=1",
		"ruleset.enabled=", // empty value allowed
		"some-rule.enabled=true",
	})
	if err != nil {
		t.Fatalf("ParseRuleOptionAssignments returned error: %v", err)
	}
	if got["some-rule"]["enabled"] != "true" {
		t.Fatalf("unexpected parsed value: %v", got)
	}
	if got["branch-protection"]["required_reviews"] != "2" {
		t.Fatalf("unexpected parsed value: %v", got)
	}
	if got["pr-reviews"]["min"] != "1" {
		t.Fatalf("unexpected parsed value: %v", got)
	}
	if got["ruleset"]["enabled"] != "" {
		t.Fatalf("expected empty string value to be preserved: %v", got)
	}
}

func TestParseRuleOptionAssignments_ErrorsOnInvalidSyntax(t *testing.T) {
	tests := []struct {
		name   string
		values []string
	}{
		{name: "missing_equals", values: []string{"a.b"}},
		{name: "missing_dot", values: []string{"ab=true"}},
		{name: "empty_rule", values: []string{".b=true"}},
		{name: "empty_opt", values: []string{"a.=true"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseRuleOptionAssignments(tt.values); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

func TestValidate_RejectsInvalidSetSyntax(t *testing.T) {
	cfg := New()
	cfg.Targeting.Repos = []string{"acme/repo"}
	cfg.Rules.Set = []string{"nope"}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestValidate_NormalizesOrgAndUserFromGitHubURLs(t *testing.T) {
	cfg := New()
	cfg.Targeting.Org = "https://github.com/acme"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}
	if cfg.Targeting.Org != "acme" {
		t.Fatalf("expected org to normalize to %q, got %q", "acme", cfg.Targeting.Org)
	}

	cfg = New()
	cfg.Targeting.User = "github.com/daneelvt"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}
	if cfg.Targeting.User != "daneelvt" {
		t.Fatalf("expected user to normalize to %q, got %q", "daneelvt", cfg.Targeting.User)
	}
}

func TestValidate_NormalizesEnterpriseFromGitHubURLs(t *testing.T) {
	cfg := New()
	cfg.Targeting.Enterprise = "https://github.com/enterprises/acme-enterprise"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}
	if cfg.Targeting.Enterprise != "acme-enterprise" {
		t.Fatalf("expected enterprise to normalize to %q, got %q", "acme-enterprise", cfg.Targeting.Enterprise)
	}
}

func TestValidate_RejectsOrgAndUserTogether(t *testing.T) {
	cfg := New()
	cfg.Targeting.Org = "acme"
	cfg.Targeting.User = "someone"
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestValidate_RejectsInvalidConsoleFormat(t *testing.T) {
	tests := []struct {
		name          string
		consoleFormat string
	}{
		{name: "empty", consoleFormat: ""},
		{name: "spaces", consoleFormat: "   "},
		{name: "unknown", consoleFormat: "yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := New()
			cfg.Targeting.Repos = []string{"acme/repo"}
			cfg.Output.ConsoleFormat = tt.consoleFormat
			if err := cfg.Validate(); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

func TestValidate_AllowsKnownConsoleFormats(t *testing.T) {
	tests := []struct {
		name          string
		consoleFormat string
	}{
		{name: "text", consoleFormat: "text"},
		{name: "json", consoleFormat: "json"},
		{name: "ndjson", consoleFormat: "ndjson"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := New()
			cfg.Targeting.Repos = []string{"acme/repo"}
			cfg.Output.ConsoleFormat = tt.consoleFormat
			if err := cfg.Validate(); err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestValidate_RejectsInvalidEvidence(t *testing.T) {
	tests := []struct {
		name     string
		evidence string
	}{
		{name: "empty", evidence: ""},
		{name: "spaces", evidence: "   "},
		{name: "unknown", evidence: "verbose"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := New()
			cfg.Targeting.Repos = []string{"acme/repo"}
			cfg.Rules.Evidence = tt.evidence
			if err := cfg.Validate(); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

func TestValidate_AllowsKnownEvidenceValues(t *testing.T) {
	tests := []struct {
		name     string
		evidence string
		want     string
	}{
		{name: "minimal", evidence: "minimal", want: "minimal"},
		{name: "standard", evidence: "standard", want: "standard"},
		{name: "full", evidence: "full", want: "full"},
		{name: "case_and_spaces", evidence: "  FULL  ", want: "full"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := New()
			cfg.Targeting.Repos = []string{"acme/repo"}
			cfg.Rules.Evidence = tt.evidence
			if err := cfg.Validate(); err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if cfg.Rules.Evidence != tt.want {
				t.Fatalf("expected evidence %q, got %q", tt.want, cfg.Rules.Evidence)
			}
		})
	}
}

func TestValidate_RejectsInvalidTargetingEnums(t *testing.T) {
	tests := []struct {
		name      string
		mutateCfg func(cfg *Config)
	}{
		{
			name: "visibility",
			mutateCfg: func(cfg *Config) {
				cfg.Targeting.Visibility = "maybe"
			},
		},
		{
			name: "archived",
			mutateCfg: func(cfg *Config) {
				cfg.Targeting.Archived = "sometimes"
			},
		},
		{
			name: "forks",
			mutateCfg: func(cfg *Config) {
				cfg.Targeting.Forks = "perhaps"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := New()
			cfg.Targeting.Repos = []string{"acme/repo"}
			tt.mutateCfg(cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

func TestValidate_NormalizesTargetingEnums(t *testing.T) {
	cfg := New()
	cfg.Targeting.Repos = []string{"acme/repo"}
	cfg.Targeting.Visibility = "  PRIVATE "
	cfg.Targeting.Archived = " INCLUDE "
	cfg.Targeting.Forks = " Only "

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Targeting.Visibility != "private" {
		t.Fatalf("expected visibility to normalize to %q, got %q", "private", cfg.Targeting.Visibility)
	}
	if cfg.Targeting.Archived != "include" {
		t.Fatalf("expected archived to normalize to %q, got %q", "include", cfg.Targeting.Archived)
	}
	if cfg.Targeting.Forks != "only" {
		t.Fatalf("expected forks to normalize to %q, got %q", "only", cfg.Targeting.Forks)
	}
}

func TestValidate_RejectsInvalidEmit(t *testing.T) {
	tests := []struct {
		name string
		emit []string
	}{
		{name: "empty", emit: []string{""}},
		{name: "unknown", emit: []string{"yaml"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := New()
			cfg.Targeting.Repos = []string{"acme/repo"}
			cfg.Output.Emit = tt.emit
			if err := cfg.Validate(); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

func TestValidate_RejectsInvalidRuntimeBounds(t *testing.T) {
	tests := []struct {
		name      string
		mutateCfg func(cfg *Config)
	}{
		{
			name: "negative_max_repos",
			mutateCfg: func(cfg *Config) {
				cfg.Targeting.MaxRepos = -1
			},
		},
		{
			name: "zero_concurrency",
			mutateCfg: func(cfg *Config) {
				cfg.Runtime.Concurrency = 0
			},
		},
		{
			name: "negative_timeout",
			mutateCfg: func(cfg *Config) {
				cfg.Runtime.Timeout = -1
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := New()
			cfg.Targeting.Repos = []string{"acme/repo"}
			tt.mutateCfg(cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}
