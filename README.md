# review-agent

A small Go service that delegates GitHub pull request reviews to Copilot (or a pluggable AI backend) when an allowlisted user requests a bot account as reviewer. Runs on the smallest DigitalOcean droplet.

## What it does

You have a bot GitHub account (e.g. `yorick-bot`). Add it as a reviewer on a pull request. If you're one of the allowlisted users, the service ships that review off to Copilot on your behalf and removes the bot from the requested reviewers so the PR only shows Copilot pending.

The reviewer backend is behind an interface — Copilot is the default; other backends (a self-hosted model, DeepSeek, etc.) can be added without touching the webhook layer.

## How it flows

```
GitHub  ──POST /webhook──►  Caddy (auto-TLS)  ──►  review-agent (:8080)
                                                       │
                                                       ├─ verify X-Hub-Signature-256 (HMAC)
                                                       ├─ event=pull_request, action=review_requested
                                                       ├─ requested_reviewer == bot_user  ?
                                                       ├─ sender ∈ allowed_users         ?
                                                       └─► Reviewer plugin (config-selected)
                                                             ├─ Copilot: DELETE bot_user → POST copilot bot
                                                             └─ AI stub: (future) diff → model → inline review comments
```

Two webhook events are subscribed to:

- `pull_request` for `review_requested` — the primary trigger.
- `issue_comment` for `/rereview` — a re-review command usable by the PR author when the active reviewer is an AI backend. Copilot's own re-review UX supersedes this, so the Copilot backend ignores it.

## Configuration

`/etc/review-agent/config.yaml`:

```yaml
bot_user: yorick-bot
listen_addr: 127.0.0.1:8080

allowed_users:
  - richard-ramos
  - gmelodie
  # ...

reviewer: copilot

reviewers:
  copilot:
    # Override the Copilot bot login if GitHub ever changes it.
    # reviewer_login: "copilot-pull-request-reviewer[bot]"

  # deepseek:
  #   api_key_env: DEEPSEEK_API_KEY
  #   model: deepseek-coder

# repo_overrides:
#   some/repo:
#     reviewer: deepseek
```

Secrets live in `/etc/review-agent/env` (mode 640, group `review-agent`):

```
WEBHOOK_SECRET=<hex, ≥32 bytes; must match the GitHub webhook secret>
GITHUB_TOKEN=<fine-grained PAT on the bot account>
```

Runtime PAT permissions (fine-grained, scoped to every repo the bot may be requested on):

- **Pull requests**: read/write (add/remove requested reviewers, and — for the AI backend — post reviews with inline comments).
- **Contents**: read (for the AI backend to fetch the diff).
- **Metadata**: read.

The bot must also be a **Triage** collaborator on each target repo. `Read` alone will not permit reviewer management.

## Deployment

The droplet runs a single 6 MB static binary behind Caddy (Let's Encrypt), managed by two systemd units:

- `review-agent.service` — the long-running HTTPS listener (bound to `127.0.0.1:8080`).
- `review-agent-updater.timer` — polls `releases/latest` every 60 s; when a new tag lands it verifies the SHA-256, runs the binary with `-check-config`, atomically swaps `/usr/local/bin/review-agent`, restarts the service, and checks `/healthz`.

No Go toolchain lives on the droplet. Cross-compiled binaries are published as GitHub releases by the workflow in `.github/workflows/release.yml`.

### Release cadence

Publishing is tag-driven, not push-driven:

```
git tag v1.2.3
git push --tags
```

The workflow builds `review-agent-linux-amd64` + `.sha256`, publishes the release, and within ~60 s the droplet has installed it. Rollback = re-run an older workflow (creates the release again with the earlier binary).

## Extending with a new reviewer backend

Every backend implements a small interface (`internal/reviewer/reviewer.go`):

```go
type Reviewer interface {
    Name() string
    Review(ctx context.Context, pr github.PullRequest) error
    SupportsReReview() bool
}
```

Register the backend in `main.go`, expose its config under `reviewers.<name>` in YAML, and select it globally with `reviewer:` or per-repo via `repo_overrides`. No webhook-layer changes required.

`internal/reviewer/ai_stub.go` is the placeholder implementation to copy from.

## Development

```
go test ./...
go build -o review-agent ./cmd/review-agent
./review-agent -config configs/config.example.yaml -check-config
```

Coverage: `config` ~90 %, `github` client ~82 %, `webhook` handler ~60 %. HMAC verification, allowlist enforcement, and reviewer dispatch all have unit tests using an in-process HTTP fake for the GitHub API — no external network required.

## Security notes

- Webhook signature verified with constant-time HMAC-SHA256 before any parsing.
- Sender allowlist enforced before any GitHub API call.
- Caddy only forwards `/webhook`; `/healthz` is localhost-only.
- systemd unit is hardened: `NoNewPrivileges`, `ProtectSystem=strict`, `ProtectHome`, `PrivateTmp`, restricted address families, memory W^X.
- UFW allows only 22 (SSH, revoke after setup), 80 (ACME), 443 (webhook).
- fail2ban `sshd` jail: 4 attempts / 10 min → 1 h ban.
- Unattended-upgrades enabled for security patches.
