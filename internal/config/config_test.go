package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoad_Valid(t *testing.T) {
	p := writeConfig(t, `
bot_user: bot
allowed_users: [alice, bob]
reviewer: copilot
reviewers:
  copilot: {}
repo_overrides:
  o/r:
    reviewer: copilot
`)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.ListenAddr != "127.0.0.1:8080" {
		t.Errorf("default listen_addr not applied, got %q", c.ListenAddr)
	}
	if !c.IsAllowed("alice") || c.IsAllowed("eve") {
		t.Errorf("IsAllowed wrong")
	}
	if c.ReviewerFor("o/r") != "copilot" {
		t.Errorf("override lookup wrong")
	}
	if c.ReviewerFor("other/repo") != "copilot" {
		t.Errorf("default reviewer wrong")
	}
}

func TestLoad_ValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want string
	}{
		{
			"missing bot_user",
			"reviewer: copilot\nreviewers: {copilot: {}}\nallowed_users: [a]\n",
			"bot_user",
		},
		{
			"missing reviewer",
			"bot_user: b\nreviewers: {copilot: {}}\nallowed_users: [a]\n",
			"reviewer is required",
		},
		{
			"reviewer not declared",
			"bot_user: b\nreviewer: deepseek\nreviewers: {copilot: {}}\nallowed_users: [a]\n",
			"not declared",
		},
		{
			"empty allowed_users",
			"bot_user: b\nreviewer: copilot\nreviewers: {copilot: {}}\n",
			"allowed_users",
		},
		{
			"override to unknown reviewer",
			"bot_user: b\nreviewer: copilot\nreviewers: {copilot: {}}\nallowed_users: [a]\nrepo_overrides: {o/r: {reviewer: nope}}\n",
			"not declared",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(writeConfig(t, tc.yaml))
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q missing %q", err, tc.want)
			}
		})
	}
}
