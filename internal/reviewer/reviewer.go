package reviewer

import (
	"context"
	"errors"

	"github.com/vacp2p/review-agent/internal/github"
)

var ErrNotImplemented = errors.New("reviewer not implemented")

type Reviewer interface {
	Name() string
	Review(ctx context.Context, pr github.PullRequest) error
	SupportsReReview() bool
}

type Registry struct {
	m map[string]Reviewer
}

func NewRegistry() *Registry { return &Registry{m: map[string]Reviewer{}} }

func (r *Registry) Register(rv Reviewer) { r.m[rv.Name()] = rv }

func (r *Registry) Get(name string) (Reviewer, bool) {
	rv, ok := r.m[name]
	return rv, ok
}
