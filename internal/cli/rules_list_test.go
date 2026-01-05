package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"repomedic/internal/data"
	"repomedic/internal/rules"

	"github.com/google/go-github/v66/github"
)

// mockRule implements rules.Rule for testing purposes
type mockRule struct {
	id          string
	title       string
	description string
}

func (m *mockRule) ID() string          { return m.id }
func (m *mockRule) Title() string       { return m.title }
func (m *mockRule) Description() string { return m.description }
func (m *mockRule) Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error) {
	return nil, nil
}
func (m *mockRule) Evaluate(ctx context.Context, repo *github.Repository, data data.DataContext) (rules.Result, error) {
	return rules.Result{}, nil
}

// mockConfigurableRule implements rules.ConfigurableRule for testing purposes
type mockConfigurableRule struct {
	mockRule
	options []rules.Option
}

func (m *mockConfigurableRule) Options() []rules.Option {
	return m.options
}

func (m *mockConfigurableRule) Configure(opts map[string]string) error {
	return nil
}

func TestPrintRule(t *testing.T) {
	tests := []struct {
		name           string
		rule           rules.Rule
		expectedOutput []string
		notExpected    []string
	}{
		{
			name: "Regular Rule",
			rule: &mockRule{
				id:          "simple-rule",
				title:       "Simple Rule",
				description: "A simple rule description",
			},
			expectedOutput: []string{
				"RULE: simple-rule",
				"Simple Rule",
				"A simple rule description",
			},
			notExpected: []string{
				"Options:",
			},
		},
		{
			name: "Configurable Rule",
			rule: &mockConfigurableRule{
				mockRule: mockRule{
					id:          "config-rule",
					title:       "Config Rule",
					description: "A configurable rule description",
				},
				options: []rules.Option{
					{
						Name:        "opt1",
						Description: "Option 1 description",
						Default:     "default1",
					},
					{
						Name:        "opt2",
						Description: "Option 2 description",
						Default:     "",
					},
				},
			},
			expectedOutput: []string{
				"RULE: config-rule",
				"Config Rule",
				"A configurable rule description",
				"Options:",
				"opt1",
				"Description: Option 1 description",
				"Default:     default1",
				"opt2",
				"Description: Option 2 description",
				"Default:     \"\"",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			printRule(buf, tt.rule)
			output := buf.String()

			for _, exp := range tt.expectedOutput {
				if !strings.Contains(output, exp) {
					t.Errorf("Expected output to contain %q, but it didn't.\nOutput:\n%s", exp, output)
				}
			}

			for _, notExp := range tt.notExpected {
				if strings.Contains(output, notExp) {
					t.Errorf("Expected output NOT to contain %q, but it did.\nOutput:\n%s", notExp, output)
				}
			}
		})
	}
}

func TestRulesListCmd(t *testing.T) {
	// Register a mock rule for testing
	mr := &mockRule{
		id:          "test-rule-list",
		title:       "Test Rule List",
		description: "This is a test rule for the list command.",
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				// Rule already registered, ignore
			}
		}()
		rules.Register(mr)
	}()

	tests := []struct {
		name           string
		quiet          bool
		expectedOutput []string
		notExpected    []string
	}{
		{
			name:  "Default Output",
			quiet: false,
			expectedOutput: []string{
				"----------------------------------------",
				"RULE: test-rule-list",
				"Test Rule List",
				"This is a test rule for the list command.",
			},
		},
		{
			name:  "Quiet Output",
			quiet: true,
			expectedOutput: []string{
				"test-rule-list",
			},
			notExpected: []string{
				"Test Rule List",
				"----------------------------------------",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flag
			rulesListQuiet = tt.quiet
			defer func() { rulesListQuiet = false }()

			buf := new(bytes.Buffer)
			rulesListCmd.SetOut(buf)

			// Execute RunE directly
			err := rulesListCmd.RunE(rulesListCmd, []string{})
			if err != nil {
				t.Fatalf("RunE() error = %v", err)
			}

			output := buf.String()
			for _, exp := range tt.expectedOutput {
				if !strings.Contains(output, exp) {
					t.Errorf("Expected output to contain %q, but it didn't.\nOutput:\n%s", exp, output)
				}
			}
			for _, notExp := range tt.notExpected {
				if strings.Contains(output, notExp) {
					t.Errorf("Expected output NOT to contain %q, but it did.\nOutput:\n%s", notExp, output)
				}
			}
		})
	}
}

func TestRulesShowCmd(t *testing.T) {
	// Register a mock rule for testing
	mr := &mockRule{
		id:          "test-rule-show",
		title:       "Test Rule Show",
		description: "This is a test rule for the show command.",
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				// Rule already registered
			}
		}()
		rules.Register(mr)
	}()

	tests := []struct {
		name           string
		args           []string
		expectedOutput []string
		expectError    bool
	}{
		{
			name: "Show Existing Rule",
			args: []string{"test-rule-show"},
			expectedOutput: []string{
				"----------------------------------------",
				"RULE: test-rule-show",
				"Test Rule Show",
				"This is a test rule for the show command.",
			},
			expectError: false,
		},
		{
			name:        "Show Non-Existent Rule",
			args:        []string{"non-existent-rule"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			rulesShowCmd.SetOut(buf)

			// Execute RunE directly
			err := rulesShowCmd.RunE(rulesShowCmd, tt.args)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				output := buf.String()
				for _, exp := range tt.expectedOutput {
					if !strings.Contains(output, exp) {
						t.Errorf("Expected output to contain %q, but it didn't.\nOutput:\n%s", exp, output)
					}
				}
			}
		})
	}
}
