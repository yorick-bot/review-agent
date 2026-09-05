package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const apiBase = "https://api.github.com"

type Client struct {
	token string
	http  *http.Client
}

func New(token string) *Client {
	return &Client{
		token: token,
		http:  &http.Client{Timeout: 15 * time.Second},
	}
}

type PullRequest struct {
	Owner  string
	Repo   string
	Number int
	Author string
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var r io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, apiBase+path, r)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("github %s %s: %d: %s", method, path, resp.StatusCode, string(b))
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func (c *Client) RequestReviewers(ctx context.Context, pr PullRequest, reviewers []string) error {
	return c.do(ctx, "POST",
		fmt.Sprintf("/repos/%s/%s/pulls/%d/requested_reviewers", pr.Owner, pr.Repo, pr.Number),
		map[string]any{"reviewers": reviewers}, nil)
}

func (c *Client) RemoveReviewers(ctx context.Context, pr PullRequest, reviewers []string) error {
	return c.do(ctx, "DELETE",
		fmt.Sprintf("/repos/%s/%s/pulls/%d/requested_reviewers", pr.Owner, pr.Repo, pr.Number),
		map[string]any{"reviewers": reviewers}, nil)
}
