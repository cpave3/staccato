#!/usr/bin/env bash
# Prints the version of the next release: the newest stable tag (vX.Y.Z) of
# the repository in the current directory, bumped by patch, minor or major.
# With no stable tag yet, it bumps v0.0.0. Used by .github/workflows/release.yml.
set -euo pipefail

bump="${1:-}"
case "$bump" in
  patch | minor | major) ;;
  *)
    echo "usage: next-version.sh patch|minor|major" >&2
    exit 2
    ;;
esac

latest="$(git tag --list 'v*' | grep -E '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$' | sort -V | tail -n 1 || true)"
latest="${latest:-v0.0.0}"
IFS=. read -r major minor patch <<<"${latest#v}"

case "$bump" in
  patch) patch=$((patch + 1)) ;;
  minor) minor=$((minor + 1)) patch=0 ;;
  major) major=$((major + 1)) minor=0 patch=0 ;;
esac
echo "v${major}.${minor}.${patch}"
