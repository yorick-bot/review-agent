package reviewer

import (
	"context"

	"github.com/vacp2p/review-agent/internal/github"
)

// Copilot delegates review to GitHub Copilot by adding it as a requested
// reviewer via the REST API, then removes the bot user from the reviewer list.
type Copilot struct {
	Client        *github.Client
	ReviewerLogin string
	BotUser       string
}

func (Copilot) Name() string           { return "copilot" }
func (Copilot) SupportsReReview() bool { return false }

func (c Copilot) Review(ctx context.Context, pr github.PullRequest) error {
	login := c.ReviewerLogin
	if login == "" {
		login = "Copilot"
	}
	if err := c.Client.RequestReviewers(ctx, pr, []string{login}); err != nil {
		return err
	}
	if c.BotUser != "" {
		_ = c.Client.RemoveReviewers(ctx, pr, []string{c.BotUser})
	}
	return nil
}
