package rules

import (
	"context"
	"repomedic/internal/data"
	"testing"

	"github.com/google/go-github/v66/github"
)

// MockRule is a simple rule for testing purposes.
type MockRule struct {
	id           string
	fail         bool
	configurable bool
	opts         map[string]string
}

func (m *MockRule) ID() string          { return m.id }
func (m *MockRule) Title() string       { return "Mock Rule" }
func (m *MockRule) Description() string { return "A mock rule" }
func (m *MockRule) Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error) {
	return nil, nil
}
func (m *MockRule) Evaluate(ctx context.Context, repo *github.Repository, dc data.DataContext) (Result, error) {
	if m.fail {
		return FailResult(repo, m.id, "failed"), nil
	}
	return PassResult(repo, m.id), nil
}

func (m *MockRule) Options() []Option {
	if !m.configurable {
		return nil
	}
	return []Option{{Name: "mock.option", Description: "A mock option"}}
}

func (m *MockRule) Configure(opts map[string]string) error {
	if !m.configurable {
		return nil
	}
	m.opts = opts
	return nil
}

func TestAllowListWrapper_Evaluate(t *testing.T) {
	repo := &github.Repository{
		FullName: github.String("org/repo"),
	}

	tests := []struct {
		name           string
		ruleFail       bool
		allowConfig    map[string]string
		expectedStatus Status
	}{
		{
			name:           "Pass - Rule passes, no allowlist",
			ruleFail:       false,
			allowConfig:    nil,
			expectedStatus: StatusPass,
		},
		{
			name:           "Fail - Rule fails, no allowlist",
			ruleFail:       true,
			allowConfig:    nil,
			expectedStatus: StatusFail,
		},
		{
			name:           "Pass - Rule fails, allowed by repo",
			ruleFail:       true,
			allowConfig:    map[string]string{"allow.repos": "org/repo"},
			expectedStatus: StatusPass,
		},
		{
			name:           "Fail - Rule fails, not allowed by repo",
			ruleFail:       true,
			allowConfig:    map[string]string{"allow.repos": "org/other"},
			expectedStatus: StatusFail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner := &MockRule{id: "mock-rule", fail: tt.ruleFail}
			wrapper := &AllowListWrapper{Rule: inner}
			if tt.allowConfig != nil {
				wrapper.Configure(tt.allowConfig)
			}

			result, err := wrapper.Evaluate(context.Background(), repo, nil)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}

			if result.Status != tt.expectedStatus {
				t.Errorf("expected status %v, got %v", tt.expectedStatus, result.Status)
			}
		})
	}
}

func TestAllowListWrapper_Options(t *testing.T) {
	// Test with non-configurable inner rule
	inner := &MockRule{id: "simple", configurable: false}
	wrapper := &AllowListWrapper{Rule: inner}
	opts := wrapper.Options()

	// Should have allowlist options (3)
	if len(opts) != 3 {
		t.Errorf("expected 3 options, got %d", len(opts))
	}

	// Test with configurable inner rule
	innerConf := &MockRule{id: "conf", configurable: true}
	wrapperConf := &AllowListWrapper{Rule: innerConf}
	optsConf := wrapperConf.Options()

	// Should have allowlist options (3) + inner options (1)
	if len(optsConf) != 4 {
		t.Errorf("expected 4 options, got %d", len(optsConf))
	}
}

func TestAllowListWrapper_Configure(t *testing.T) {
	inner := &MockRule{id: "conf", configurable: true}
	wrapper := &AllowListWrapper{Rule: inner}

	config := map[string]string{
		"allow.repos": "org/repo",
		"mock.option": "value",
	}

	err := wrapper.Configure(config)
	if err != nil {
		t.Fatalf("Configure error: %v", err)
	}

	// Check allowlist configured
	if !wrapper.allowList.Repos["org/repo"] {
		t.Error("allowlist not configured correctly")
	}

	// Check inner rule configured
	if inner.opts["mock.option"] != "value" {
		t.Error("inner rule not configured correctly")
	}
}
