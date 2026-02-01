#!/usr/bin/env bash
#MISE description="Bump patch version and release"
set -euo pipefail

# Get latest tag, default to v0.0.0 if none exists
latest_tag=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")

# Parse version components
version="${latest_tag#v}"
IFS='.' read -r major minor patch <<<"$version"

# Bump patch
new_version="v${major}.${minor}.$((patch + 1))"

echo "Bumping version: ${latest_tag} → ${new_version}"

# Create and push tag
git tag -a "$new_version" -m "Release $new_version"
git push origin "$new_version"

# Run goreleaser (expects GORELEASER_GITHUB_TOKEN env var)
GITHUB_TOKEN="${GORELEASER_GITHUB_TOKEN}" goreleaser release --clean

# Clear mise cache so upgrade detects new version immediately
mise cache clear

echo "Released ${new_version}"
