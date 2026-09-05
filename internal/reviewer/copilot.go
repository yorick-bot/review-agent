package reviewer

import (
	"context"
	"log/slog"

	"github.com/vacp2p/review-agent/internal/github"
)

type Copilot struct {
	Client        *github.Client
	ReviewerLogin string
	BotUser       string
	Log           *slog.Logger
}

func (Copilot) Name() string           { return "copilot" }
func (Copilot) SupportsReReview() bool { return false }

func (c Copilot) Review(ctx context.Context, pr github.PullRequest) error {
	login := c.ReviewerLogin
	if login == "" {
		login = "copilot-pull-request-reviewer[bot]"
	}
	if err := c.Client.RequestReviewers(ctx, pr, []string{login}); err != nil {
		return err
	}
	if c.BotUser != "" {
		if err := c.Client.RemoveReviewers(ctx, pr, []string{c.BotUser}); err != nil && c.Log != nil {
			c.Log.Warn("remove bot from reviewers failed",
				"repo", pr.Owner+"/"+pr.Repo, "pr", pr.Number, "err", err)
		}
	}
	return nil
}
