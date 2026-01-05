package rules

type Status string

const (
	StatusPass    Status = "PASS"
	StatusFail    Status = "FAIL"
	StatusSkipped Status = "SKIPPED"
	StatusError   Status = "ERROR"
)

type Result struct {
	RuleID  string `json:"rule_id"`
	Repo    string `json:"repo"`
	Status  Status `json:"status"`
	Message string `json:"message,omitempty"`
	// Evidence contains simple key-value string pairs supporting the result.
	Evidence map[string]string `json:"evidence,omitempty"`
	// Metadata contains structured data supporting the result (e.g. lists, counts).
	Metadata map[string]any `json:"metadata,omitempty"`
	WrongID  string         `json:"wrong_id,omitempty"`
}
