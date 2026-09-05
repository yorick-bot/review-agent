package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vacp2p/review-agent/internal/config"
	"github.com/vacp2p/review-agent/internal/github"
	"github.com/vacp2p/review-agent/internal/reviewer"
)

type fakeReviewer struct {
	calls int
	last  github.PullRequest
	name  string
}

func (f *fakeReviewer) Name() string           { return f.name }
func (*fakeReviewer) SupportsReReview() bool   { return true }
func (f *fakeReviewer) Review(_ context.Context, pr github.PullRequest) error {
	f.calls++
	f.last = pr
	return nil
}

func newHandler(t *testing.T, fake *fakeReviewer) *Handler {
	t.Helper()
	cfg := &config.Config{
		BotUser:      "bot",
		AllowedUsers: []string{"alice"},
		Reviewer:     "copilot",
		Reviewers:    map[string]map[string]string{"copilot": {}},
	}
	reg := reviewer.NewRegistry()
	reg.Register(fake)
	return &Handler{
		Secret:   []byte("s3cret"),
		Config:   cfg,
		Registry: reg,
		Log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func signReq(t *testing.T, secret, body []byte, event string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	m := hmac.New(sha256.New, secret)
	m.Write(body)
	req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(m.Sum(nil)))
	req.Header.Set("X-GitHub-Event", event)
	req.Header.Set("Content-Type", "application/json")
	return req
}

const prPayloadAllowed = `{
  "action":"review_requested",
  "number":42,
  "pull_request":{"number":42,"user":{"login":"charlie"}},
  "requested_reviewer":{"login":"bot"},
  "repository":{"full_name":"o/r","owner":{"login":"o"},"name":"r"},
  "sender":{"login":"alice"}
}`

const prPayloadRejected = `{
  "action":"review_requested",
  "number":42,
  "pull_request":{"number":42,"user":{"login":"charlie"}},
  "requested_reviewer":{"login":"bot"},
  "repository":{"full_name":"o/r","owner":{"login":"o"},"name":"r"},
  "sender":{"login":"eve"}
}`

const prPayloadOtherReviewer = `{
  "action":"review_requested",
  "number":42,
  "pull_request":{"number":42,"user":{"login":"charlie"}},
  "requested_reviewer":{"login":"someone-else"},
  "repository":{"full_name":"o/r","owner":{"login":"o"},"name":"r"},
  "sender":{"login":"alice"}
}`

func TestHandler_PR_AllowedDispatches(t *testing.T) {
	fake := &fakeReviewer{name: "copilot"}
	h := newHandler(t, fake)
	req := signReq(t, h.Secret, []byte(prPayloadAllowed), "pull_request")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if fake.calls != 1 {
		t.Fatalf("expected 1 call, got %d", fake.calls)
	}
	if fake.last.Owner != "o" || fake.last.Repo != "r" || fake.last.Number != 42 {
		t.Fatalf("unexpected PR: %+v", fake.last)
	}
}

func TestHandler_PR_RejectedNotAllowlisted(t *testing.T) {
	fake := &fakeReviewer{name: "copilot"}
	h := newHandler(t, fake)
	req := signReq(t, h.Secret, []byte(prPayloadRejected), "pull_request")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if fake.calls != 0 {
		t.Fatalf("reviewer must not run for non-allowlisted sender")
	}
}

func TestHandler_PR_IgnoresOtherReviewer(t *testing.T) {
	fake := &fakeReviewer{name: "copilot"}
	h := newHandler(t, fake)
	req := signReq(t, h.Secret, []byte(prPayloadOtherReviewer), "pull_request")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if fake.calls != 0 {
		t.Fatalf("must ignore when requested_reviewer is not bot_user")
	}
}

func TestHandler_BadSignature(t *testing.T) {
	fake := &fakeReviewer{name: "copilot"}
	h := newHandler(t, fake)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader([]byte(prPayloadAllowed)))
	req.Header.Set("X-Hub-Signature-256", "sha256=00")
	req.Header.Set("X-GitHub-Event", "pull_request")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	if fake.calls != 0 {
		t.Fatalf("reviewer must not run on bad signature")
	}
}

const commentReReview = `{
  "action":"created",
  "issue":{"number":7,"pull_request":{},"user":{"login":"charlie"}},
  "comment":{"body":"/rereview please","user":{"login":"charlie"}},
  "repository":{"full_name":"o/r","owner":{"login":"o"},"name":"r"},
  "sender":{"login":"charlie"}
}`

const commentReReviewByOther = `{
  "action":"created",
  "issue":{"number":7,"pull_request":{},"user":{"login":"charlie"}},
  "comment":{"body":"/rereview please","user":{"login":"eve"}},
  "repository":{"full_name":"o/r","owner":{"login":"o"},"name":"r"},
  "sender":{"login":"eve"}
}`

func TestHandler_ReReview_AuthorTriggers(t *testing.T) {
	fake := &fakeReviewer{name: "copilot"}
	h := newHandler(t, fake)
	// Copilot returns SupportsReReview=false in real impl; here fake=true.
	req := signReq(t, h.Secret, []byte(commentReReview), "issue_comment")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if fake.calls != 1 {
		t.Fatalf("expected 1 call, got %d", fake.calls)
	}
}

func TestHandler_ReReview_NonAuthorIgnored(t *testing.T) {
	fake := &fakeReviewer{name: "copilot"}
	h := newHandler(t, fake)
	req := signReq(t, h.Secret, []byte(commentReReviewByOther), "issue_comment")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if fake.calls != 0 {
		t.Fatalf("only PR author may /rereview")
	}
}
