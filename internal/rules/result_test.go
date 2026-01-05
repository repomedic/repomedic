package rules

import (
	"encoding/json"
	"testing"
)

func TestResultSerialization(t *testing.T) {
	r := Result{
		RuleID:  "test-rule",
		Repo:    "acme/repo",
		Status:  StatusFail,
		Message: "Something is wrong",
		Evidence: map[string]string{
			"key": "value",
		},
		WrongID: "wrong-123",
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	expected := `{"rule_id":"test-rule","repo":"acme/repo","status":"FAIL","message":"Something is wrong","evidence":{"key":"value"},"wrong_id":"wrong-123"}`
	if string(data) != expected {
		t.Errorf("Expected %s, got %s", expected, string(data))
	}
}
