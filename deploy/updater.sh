#!/usr/bin/env bash
# Poll GitHub Releases for the latest tag; if new, download, verify, swap, restart.
# Run via systemd timer. Idempotent — safe to call every minute.
set -euo pipefail

REPO="${REPO:?REPO env required, e.g. vacp2p/review-agent}"
BIN_PATH="${BIN_PATH:-/usr/local/bin/review-agent}"
STATE_DIR="${STATE_DIR:-/var/lib/review-agent}"
SERVICE="${SERVICE:-review-agent}"
CONFIG="${CONFIG:-/etc/review-agent/config.yaml}"
ENV_FILE="${ENV_FILE:-/etc/review-agent/env}"
ASSET="review-agent-linux-amd64"
API="https://api.github.com/repos/${REPO}/releases/latest"

mkdir -p "${STATE_DIR}"
current="$(cat "${STATE_DIR}/version" 2>/dev/null || true)"

auth=()
if [[ -n "${GITHUB_TOKEN:-}" ]]; then
    auth=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
fi

latest="$(curl -sSf "${auth[@]}" \
    -H 'Accept: application/vnd.github+json' \
    -H 'X-GitHub-Api-Version: 2022-11-28' \
    "${API}" | jq -r .tag_name)"

if [[ -z "${latest}" || "${latest}" == "null" ]]; then
    echo "no release found" >&2
    exit 1
fi
if [[ "${latest}" == "${current}" ]]; then
    exit 0
fi

echo "updating ${current:-<none>} -> ${latest}"
tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT

base="https://github.com/${REPO}/releases/download/${latest}"
curl -sSfL "${auth[@]}" -o "${tmp}/${ASSET}"        "${base}/${ASSET}"
curl -sSfL "${auth[@]}" -o "${tmp}/${ASSET}.sha256" "${base}/${ASSET}.sha256"

( cd "${tmp}" && sha256sum -c "${ASSET}.sha256" )
chmod +x "${tmp}/${ASSET}"

# Validate config + required env against the new binary before swapping.
set -a; . "${ENV_FILE}"; set +a
"${tmp}/${ASSET}" -config "${CONFIG}" -check-config

install -m 755 "${tmp}/${ASSET}" "${BIN_PATH}"
systemctl restart "${SERVICE}"

# Health check — service listens on 127.0.0.1:8080.
for i in 1 2 3 4 5; do
    sleep 1
    if curl -sSf -o /dev/null http://127.0.0.1:8080/healthz; then
        echo "${latest}" > "${STATE_DIR}/version"
        echo "deployed ${latest}"
        exit 0
    fi
done

echo "service failed healthz after restart" >&2
exit 1
