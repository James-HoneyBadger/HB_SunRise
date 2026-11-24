#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2020-2026 Honey Badger Universe
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Builds HB SunRise as a standalone program for Linux, Windows and macOS.
# Usage: scripts/build-hb-sunrise.sh [output-dir]   (default: dist)

set -euo pipefail

cd "$(dirname "$0")/.."

out="${1:-dist}"
mkdir -p "$out"

# The binary reports the tag it was built from, so a downloaded binary and the
# release it came from agree. The git tag is the only version source, so the
# tag and the release can never disagree. The commit must be exactly on a tag:
# a build from somewhere in history has no release to agree with, so it
# reports "dev", which is what a plain "go build" reports too.
version="$(git describe --tags --exact-match 2>/dev/null || true)"

# Tags are written v0.1.0; the program prints the number alone.
version="${version#v}"

if [ -z "$version" ]; then
    # A release must never ship binaries labelled "dev", so the workflow that
    # publishes them asks for this to be fatal.
    if [ "${HB_SUNRISE_REQUIRE_VERSION:-}" = "1" ]; then
        echo "error: HEAD is not on a tag, so the binaries would report \"dev\"; tag the commit first" >&2
        exit 1
    fi

    echo "note: HEAD is not on a tag; the binaries will report \"dev\"" >&2
fi

ldflags="-s -w"

if [ -n "$version" ]; then
    ldflags="$ldflags -X main.version=$version"
fi

for target in linux/amd64 linux/arm64 windows/amd64 darwin/amd64 darwin/arm64; do
    os="${target%/*}"
    arch="${target#*/}"
    ext=""
    [ "$os" = "windows" ] && ext=".exe"

    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
        go build -trimpath -ldflags "$ldflags" -o "$out/hb-sunrise-$os-$arch$ext" ./cmd/hb-sunrise

    echo "built $out/hb-sunrise-$os-$arch$ext"
done
