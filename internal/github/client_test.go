package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, h http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	c := &Client{token: "tok", http: srv.Client()}
	// Redirect apiBase to test server by patching the base URL used in do().
	// Since apiBase is a const, we override by wrapping through the test server.
	c.http = srv.Client()
	return c, srv
}

// The client uses a const apiBase, so we can't redirect it via a field.
// Instead, we build a request-level interceptor by shadowing the transport.

type rewriteTransport struct {
	base string
	rt   http.RoundTripper
}

func (r rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	u := *req.URL
	u.Scheme = "http"
	u.Host = strings.TrimPrefix(r.base, "http://")
	req2 := req.Clone(req.Context())
	req2.URL = &u
	return r.rt.RoundTrip(req2)
}

func TestRequestReviewers_Success(t *testing.T) {
	var got struct {
		Reviewers []string `json:"reviewers"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if r.URL.Path != "/repos/o/r/pulls/42/requested_reviewers" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(201)
	}))
	defer srv.Close()

	c := &Client{token: "tok", http: &http.Client{Transport: rewriteTransport{base: srv.URL, rt: http.DefaultTransport}}}
	err := c.RequestReviewers(context.Background(),
		PullRequest{Owner: "o", Repo: "r", Number: 42}, []string{"Copilot"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Reviewers) != 1 || got.Reviewers[0] != "Copilot" {
		t.Fatalf("payload = %+v", got)
	}
}

func TestRequestReviewers_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"nope"}`, http.StatusForbidden)
	}))
	defer srv.Close()

	c := &Client{token: "tok", http: &http.Client{Transport: rewriteTransport{base: srv.URL, rt: http.DefaultTransport}}}
	err := c.RequestReviewers(context.Background(),
		PullRequest{Owner: "o", Repo: "r", Number: 1}, []string{"Copilot"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Fatalf("error should mention status: %v", err)
	}
}

func TestRemoveReviewers(t *testing.T) {
	seen := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = true
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s", r.Method)
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := &Client{token: "tok", http: &http.Client{Transport: rewriteTransport{base: srv.URL, rt: http.DefaultTransport}}}
	if err := c.RemoveReviewers(context.Background(),
		PullRequest{Owner: "o", Repo: "r", Number: 3}, []string{"bot"}); err != nil {
		t.Fatal(err)
	}
	if !seen {
		t.Fatal("server not called")
	}
}
