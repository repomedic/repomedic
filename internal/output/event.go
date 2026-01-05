package output

import "repomedic/internal/rules"

// Event is a lifecycle record for NDJSON streaming output.
//
// In NDJSON mode, sinks emit Events (one JSON object per line), including:
// - run.started
// - repo.started
// - rule.result
// - repo.finished
// - run.finished
//
// JSON mode remains an aggregate of rules.Result values.
type Event struct {
	Type string `json:"type"`
	Repo string `json:"repo,omitempty"`
	*rules.Result
	Repos    int `json:"repos,omitempty"`
	Rules    int `json:"rules,omitempty"`
	ExitCode int `json:"exit_code,omitempty"`
}

func eventFromResult(r rules.Result) Event {
	return Event{Type: "rule.result", Repo: r.Repo, Result: &r}
}
