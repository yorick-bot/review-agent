package reviewer

import (
	"context"

	"github.com/vacp2p/review-agent/internal/github"
)

// AIStub is a placeholder for future AI-backed reviewers (deepseek, etc.).
// It satisfies the Reviewer interface so config can declare backends today
// and an implementation can be dropped in later without touching dispatch.
type AIStub struct{ NameStr string }

func (a AIStub) Name() string         { return a.NameStr }
func (AIStub) SupportsReReview() bool { return true }
func (AIStub) Review(context.Context, github.PullRequest) error {
	return ErrNotImplemented
}
