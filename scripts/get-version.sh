#!/bin/bash
# get-version.sh - Shared version detection for all build scripts
# Usage: source this script and call get_version

get_version() {
    local root="${1:-.}"
    cd "$root" || return 1

    # Check if we're in a git repo
    if ! git rev-parse --git-dir >/dev/null 2>&1; then
        echo "dev"
        return 0
    fi

    # Try to get exact tag for current commit (annotated or lightweight)
    # This finds tags that point exactly to HEAD
    local tag=$(git describe --exact-match --tags HEAD 2>/dev/null | head -1)

    if [ -n "$tag" ]; then
        # Strip leading 'v' if present (e.g., v0.2.120 -> 0.2.120)
        echo "${tag#v}"
        return 0
    fi

    # No exact tag - use short SHA with prefix
    local sha=$(git rev-parse --short=8 HEAD 2>/dev/null)
    if [ -n "$sha" ]; then
        echo "dev-${sha}"
        return 0
    fi

    echo "dev"
}

# Also export a function that gets version from wails.json as fallback
get_version_from_wails() {
    local root="${1:-.}"
    local wails_json="$root/gui/wails.json"
    if [ -f "$wails_json" ]; then
        sed -n 's/.*"productVersion"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$wails_json" | head -1
    fi
}

# Main: if called directly (not sourced), print version
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
    get_version "$@"
fi