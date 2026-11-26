#!/usr/bin/env sh
# SPDX-FileCopyrightText: 2020-2026 Honey Badger Universe
# SPDX-License-Identifier: GPL-3.0-or-later

# Fails when analytics, crash-reporting or telemetry endpoints appear in the
# code that ships to users, or when that code makes network connections. HB
# SunRise is completely offline: it must not phone home.

set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

cd "$ROOT_DIR"

# Hosts and SDK identifiers used for product analytics or crash reporting.
# Keep this list to services that exist to collect user behaviour.
PATTERN='google-analytics\.com|googletagmanager\.com|analytics\.google\.com|hm\.baidu\.com|umeng\.com|mixpanel\.com|posthog\.com|amplitude\.com|hotjar\.com|fullstory\.com|segment\.(io|com)|bugsnag\.com|sentry\.io|appcenter\.ms|datadoghq\.com|newrelic\.com|@sentry/|firebase-analytics'

# Network APIs in Go code. The Wails library itself is out of scope; this
# checks the code this repository ships, including the dependency manifests.
GO_NET_PATTERN='net/http|net\.Dial|websocket|http\.Get|http\.Post|http\.Client|httputil'

# Network calls in the frontend code, which runs inside the webview.
WEB_NET_PATTERN='fetch\(|XMLHttpRequest|WebSocket|navigator\.sendBeacon'

SKIP_DIRS='--exclude-dir=node_modules --exclude-dir=dist --exclude-dir=vendor'
GO_FILES="--include=*.go"
WEB_FILES="--include=*.ts --include=*.tsx --include=*.js --include=*.html"

failures=0

scan() {
    # scan <label> <pattern> <include flags> [paths...]
    label="$1"
    pattern="$2"
    includes="$3"
    shift 3

    # GNU grep 3.11 ignores --include once a file-level --exclude is also
    # given, so only --exclude-dir flags are used here.
    # shellcheck disable=SC2086
    matches=$(grep -rInE $SKIP_DIRS $includes "$pattern" "$@" 2>/dev/null || true)

    if [ -n "$matches" ]; then
        echo "$label failed:"
        echo ""
        echo "$matches"
        echo ""
        failures=$((failures + 1))
    fi
}

for candidate in pkg cmd desktop; do
    if [ -e "$candidate" ]; then
        scan "Telemetry check" "$PATTERN" "$GO_FILES $WEB_FILES" "$candidate"
        scan "Offline check (Go)" "$GO_NET_PATTERN" "$GO_FILES" "$candidate"
    fi
done

if [ -e desktop/frontend/src ]; then
    scan "Offline check (frontend)" "$WEB_NET_PATTERN" "$WEB_FILES" desktop/frontend/src
fi

if [ "$failures" -gt 0 ]; then
    echo ""
    echo "HB SunRise is completely offline and must not phone home. Remove the"
    echo "reference, or extend scripts/check-no-telemetry.sh if it is a false"
    echo "positive (for example a user-configured provider)."
    exit 1
fi

echo "Offline check passed: no telemetry endpoints and no network calls."
