#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

if ! command -v k6 >/dev/null 2>&1; then
    echo "k6 is not installed. See https://grafana.com/docs/k6/latest/set-up/install-k6/" >&2
    exit 1
fi

export BASE_URL="${BASE_URL:-http://localhost:4333}"

export K6_WEB_DASHBOARD="${K6_WEB_DASHBOARD:-true}"
export K6_WEB_DASHBOARD_OPEN="${K6_WEB_DASHBOARD_OPEN:-false}"

if ! curl -fsS -o /dev/null --max-time 5 "${BASE_URL}/livez"; then
    echo "No server responding at ${BASE_URL}/livez" >&2
    echo "Start it with 'go run .' (the copied .env binds 127.0.0.1:4333) or 'docker compose --profile app up -d --build' (host 4333)." >&2
    exit 1
fi

echo "Load testing ${BASE_URL}..."
exec k6 run "$@" read-endpoints.js
