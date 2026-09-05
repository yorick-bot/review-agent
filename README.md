# review-agent

Small Go service that runs on a DigitalOcean droplet and delegates GitHub PR reviews to Copilot (default) or a pluggable AI backend.

## How it works

- Deployed as a systemd service behind Caddy (auto-TLS via Let's Encrypt).
- GitHub webhooks fire on `pull_request.review_requested`; when the bot user is requested by an allowlisted sender, the service asks Copilot to review the PR and removes itself from the requested reviewers list.
- Releases are cut by GitHub Actions on tags matching `v*`; the droplet polls `releases/latest` every 60 seconds and installs new binaries after a config check and health probe.

## Configuration

See [`configs/config.example.yaml`](configs/config.example.yaml).
