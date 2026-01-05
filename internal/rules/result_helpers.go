package rules

import "github.com/google/go-github/v66/github"

func RepoFullName(repo *github.Repository) string {
	if repo == nil {
		return ""
	}
	return repo.GetFullName()
}

func NewResult(repo *github.Repository, ruleID string, status Status, message string) Result {
	res := Result{
		Status: status,
		Repo:   RepoFullName(repo),
		RuleID: ruleID,
	}
	if message != "" {
		res.Message = message
	}
	return res
}

func PassResult(repo *github.Repository, ruleID string) Result {
	return NewResult(repo, ruleID, StatusPass, "")
}

func PassResultWithMessage(repo *github.Repository, ruleID string, message string) Result {
	return NewResult(repo, ruleID, StatusPass, message)
}

func FailResult(repo *github.Repository, ruleID string, message string) Result {
	return NewResult(repo, ruleID, StatusFail, message)
}

func ErrorResult(repo *github.Repository, ruleID string, message string) Result {
	return NewResult(repo, ruleID, StatusError, message)
}

func SkippedResult(repo *github.Repository, ruleID string, message string) Result {
	return NewResult(repo, ruleID, StatusSkipped, message)
}

func PassResultWithMetadata(repo *github.Repository, ruleID string, message string, metadata map[string]any) Result {
	res := NewResult(repo, ruleID, StatusPass, message)
	res.Metadata = metadata
	return res
}

func FailResultWithMetadata(repo *github.Repository, ruleID string, message string, metadata map[string]any) Result {
	res := NewResult(repo, ruleID, StatusFail, message)
	res.Metadata = metadata
	return res
}
