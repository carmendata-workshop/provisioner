#!/bin/bash

# Automatic Semantic Versioning based on Conventional Commits
# Usage: ./scripts/version.sh [base_version]

set -e

# Get the last tag or use provided base version
if [ -n "$1" ]; then
    LAST_VERSION="$1"
elif git describe --tags --abbrev=0 >/dev/null 2>&1; then
    LAST_VERSION=$(git describe --tags --abbrev=0)
else
    LAST_VERSION="v0.0.0"
fi

echo "🔍 Last version: $LAST_VERSION" >&2

# Remove 'v' prefix for version parsing
VERSION_NUMBER=${LAST_VERSION#v}

# Parse semantic version (major.minor.patch)
IFS='.' read -r MAJOR MINOR PATCH <<< "$VERSION_NUMBER"

# If this is the very first version (v0.0.0), start with MVP version
if [ "$LAST_VERSION" = "v0.0.0" ]; then
    echo "🎯 First release detected - starting with MVP version v0.1.0" >&2
    MAJOR=0
    MINOR=1
    PATCH=0
fi

# Get commits since last tag
if [ "$LAST_VERSION" = "v0.0.0" ]; then
    # If no previous tags, get all commits
    COMMITS=$(git log --oneline --pretty=format:"%s")
else
    # Get commits since last tag
    COMMITS=$(git log ${LAST_VERSION}..HEAD --oneline --pretty=format:"%s")
fi

if [ -z "$COMMITS" ]; then
    echo "📝 No new commits since $LAST_VERSION" >&2
    echo "$LAST_VERSION"
    exit 0
fi

echo "📋 Analyzing commits since $LAST_VERSION:" >&2

# Initialize version bump flags
BUMP_MAJOR=false
BUMP_MINOR=false
BUMP_PATCH=false

# Analyze each commit message
while IFS= read -r commit; do
    echo "  - $commit" >&2

    # Check for breaking changes
    if [[ "$commit" =~ ^feat!:|^fix!:|^perf!: ]] || [[ "$commit" =~ BREAKING\ CHANGE ]]; then
        echo "    → 💥 BREAKING CHANGE detected" >&2
        BUMP_MAJOR=true
    # Check for features
    elif [[ "$commit" =~ ^feat ]]; then
        echo "    → ✨ Feature detected" >&2
        BUMP_MINOR=true
    # Check for fixes
    elif [[ "$commit" =~ ^fix ]]; then
        echo "    → 🐛 Fix detected" >&2
        BUMP_PATCH=true
    # Check for performance improvements
    elif [[ "$commit" =~ ^perf ]]; then
        echo "    → ⚡ Performance improvement detected" >&2
        BUMP_PATCH=true
    # Other types don't bump version
    elif [[ "$commit" =~ ^(docs|style|refactor|test|chore|ci|build) ]]; then
        echo "    → 📝 Non-version-bumping change" >&2
    else
        echo "    → ⚠️  Non-conventional commit (treating as patch)" >&2
        BUMP_PATCH=true
    fi
done <<< "$COMMITS"

# Calculate new version
if [ "$LAST_VERSION" = "v0.0.0" ]; then
    # First release - use MVP version regardless of commits
    NEW_MAJOR=0
    NEW_MINOR=1
    NEW_PATCH=0
    BUMP_TYPE="initial MVP release"
elif [ "$BUMP_MAJOR" = true ]; then
    NEW_MAJOR=$((MAJOR + 1))
    NEW_MINOR=0
    NEW_PATCH=0
    BUMP_TYPE="major"
elif [ "$BUMP_MINOR" = true ]; then
    NEW_MAJOR=$MAJOR
    NEW_MINOR=$((MINOR + 1))
    NEW_PATCH=0
    BUMP_TYPE="minor"
elif [ "$BUMP_PATCH" = true ]; then
    NEW_MAJOR=$MAJOR
    NEW_MINOR=$MINOR
    NEW_PATCH=$((PATCH + 1))
    BUMP_TYPE="patch"
else
    echo "📝 No version-bumping changes found" >&2
    echo "$LAST_VERSION"
    exit 0
fi

NEW_VERSION="v${NEW_MAJOR}.${NEW_MINOR}.${NEW_PATCH}"

echo "🚀 Version bump: $LAST_VERSION → $NEW_VERSION ($BUMP_TYPE)" >&2
echo "$NEW_VERSION"