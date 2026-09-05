package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	BotUser       string                       `yaml:"bot_user"`
	ListenAddr    string                       `yaml:"listen_addr"`
	AllowedUsers  []string                     `yaml:"allowed_users"`
	Reviewer      string                       `yaml:"reviewer"`
	Reviewers     map[string]map[string]string `yaml:"reviewers"`
	RepoOverrides map[string]RepoOverride      `yaml:"repo_overrides"`
}

type RepoOverride struct {
	Reviewer string `yaml:"reviewer"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c := &Config{ListenAddr: "127.0.0.1:8080"}
	if err := yaml.Unmarshal(b, c); err != nil {
		return nil, err
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) validate() error {
	if c.BotUser == "" {
		return fmt.Errorf("bot_user is required")
	}
	if c.Reviewer == "" {
		return fmt.Errorf("reviewer is required")
	}
	if _, ok := c.Reviewers[c.Reviewer]; !ok {
		return fmt.Errorf("reviewer %q not declared under reviewers", c.Reviewer)
	}
	if len(c.AllowedUsers) == 0 {
		return fmt.Errorf("allowed_users must be non-empty")
	}
	for repo, o := range c.RepoOverrides {
		if o.Reviewer == "" {
			continue
		}
		if _, ok := c.Reviewers[o.Reviewer]; !ok {
			return fmt.Errorf("repo_overrides[%s]: reviewer %q not declared", repo, o.Reviewer)
		}
	}
	return nil
}

func (c *Config) IsAllowed(login string) bool {
	for _, u := range c.AllowedUsers {
		if u == login {
			return true
		}
	}
	return false
}

func (c *Config) ReviewerFor(repoFullName string) string {
	if o, ok := c.RepoOverrides[repoFullName]; ok && o.Reviewer != "" {
		return o.Reviewer
	}
	return c.Reviewer
}
