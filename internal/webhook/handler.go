package webhook

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/vacp2p/review-agent/internal/config"
	"github.com/vacp2p/review-agent/internal/github"
	"github.com/vacp2p/review-agent/internal/reviewer"
)

const maxBodyBytes = 1 << 20 // 1 MiB

type Handler struct {
	Secret   []byte
	Config   *config.Config
	Registry *reviewer.Registry
	Log      *slog.Logger
}

type user struct {
	Login string `json:"login"`
}

type repo struct {
	FullName string `json:"full_name"`
	Owner    user   `json:"owner"`
	Name     string `json:"name"`
}

type pullRequest struct {
	Number int  `json:"number"`
	User   user `json:"user"`
}

type prEvent struct {
	Action            string      `json:"action"`
	Number            int         `json:"number"`
	PullRequest       pullRequest `json:"pull_request"`
	RequestedReviewer *user       `json:"requested_reviewer"`
	Repository        repo        `json:"repository"`
	Sender            user        `json:"sender"`
}

type issueRef struct {
	Number      int       `json:"number"`
	PullRequest *struct{} `json:"pull_request"`
	User        user      `json:"user"`
}

type comment struct {
	Body string `json:"body"`
	User user   `json:"user"`
}

type commentEvent struct {
	Action     string   `json:"action"`
	Issue      issueRef `json:"issue"`
	Comment    comment  `json:"comment"`
	Repository repo     `json:"repository"`
	Sender     user     `json:"sender"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" && r.Method == http.MethodGet {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return
	}
	if r.URL.Path != "/webhook" || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		http.Error(w, "read", http.StatusBadRequest)
		return
	}
	if err := Verify(h.Secret, r.Header.Get("X-Hub-Signature-256"), body); err != nil {
		http.Error(w, "signature", http.StatusUnauthorized)
		return
	}
	switch r.Header.Get("X-GitHub-Event") {
	case "pull_request":
		h.handlePR(r.Context(), body)
	case "issue_comment":
		h.handleComment(r.Context(), body)
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) handlePR(ctx context.Context, body []byte) {
	var e prEvent
	if err := json.Unmarshal(body, &e); err != nil {
		h.Log.Warn("pr event decode", "err", err)
		return
	}
	if e.Action != "review_requested" || e.RequestedReviewer == nil {
		return
	}
	if e.RequestedReviewer.Login != h.Config.BotUser {
		return
	}
	if !h.Config.IsAllowed(e.Sender.Login) {
		h.Log.Info("rejected: sender not allowed",
			"sender", e.Sender.Login, "repo", e.Repository.FullName, "pr", e.Number)
		return
	}
	name := h.Config.ReviewerFor(e.Repository.FullName)
	rv, ok := h.Registry.Get(name)
	if !ok {
		h.Log.Error("no reviewer registered", "name", name)
		return
	}
	pr := github.PullRequest{
		Owner:  e.Repository.Owner.Login,
		Repo:   e.Repository.Name,
		Number: e.Number,
		Author: e.PullRequest.User.Login,
	}
	if err := rv.Review(ctx, pr); err != nil {
		h.Log.Error("review failed",
			"reviewer", name, "repo", e.Repository.FullName, "pr", e.Number, "err", err)
		return
	}
	h.Log.Info("review dispatched",
		"reviewer", name, "repo", e.Repository.FullName, "pr", e.Number, "by", e.Sender.Login)
}

func (h *Handler) handleComment(ctx context.Context, body []byte) {
	var e commentEvent
	if err := json.Unmarshal(body, &e); err != nil {
		h.Log.Warn("comment event decode", "err", err)
		return
	}
	if e.Action != "created" || e.Issue.PullRequest == nil {
		return
	}
	if !strings.HasPrefix(strings.TrimSpace(e.Comment.Body), "/rereview") {
		return
	}
	if e.Sender.Login != e.Issue.User.Login {
		return
	}
	name := h.Config.ReviewerFor(e.Repository.FullName)
	rv, ok := h.Registry.Get(name)
	if !ok || !rv.SupportsReReview() {
		return
	}
	pr := github.PullRequest{
		Owner:  e.Repository.Owner.Login,
		Repo:   e.Repository.Name,
		Number: e.Issue.Number,
		Author: e.Issue.User.Login,
	}
	if err := rv.Review(ctx, pr); err != nil {
		h.Log.Error("re-review failed",
			"reviewer", name, "repo", e.Repository.FullName, "pr", e.Issue.Number, "err", err)
		return
	}
	h.Log.Info("re-review dispatched",
		"reviewer", name, "repo", e.Repository.FullName, "pr", e.Issue.Number)
}
