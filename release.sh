#!/bin/bash
set -e # Exit immediately if a command exits with a non-zero status.

# --- Configuration ---
# This script creates and pushes a single release tag for a specific activity using goreleaser.
# It builds and attaches both CP and CEDT binaries into a single release.
#
# Usage: ./release.sh <activity_name> <version>
#
# Example: ./release.sh activity1 v0.1.0
#
# This will create and publish a GitHub release with tag:
# - v0.1.0-activity1

# --- Input Validation ---
if [ -z "$1" ] || [ -z "$2" ]; then
    echo "Error: Activity name and version are required."
    echo "Usage: $0 <activity_name> <version>"
    echo "Example: $0 activity1 v0.1.0"
    exit 1
fi

ACTIVITY_NAME=$1
VERSION=$2
TAG="${VERSION}-${ACTIVITY_NAME}"

if [ ! -d "$ACTIVITY_NAME" ]; then
    echo "Error: Activity directory './${ACTIVITY_NAME}' not found."
    exit 1
fi

# Check if goreleaser is installed
if ! command -v goreleaser &> /dev/null; then
    echo "Error: goreleaser is not installed. Please install it first."
    echo "See: https://goreleaser.com/install/"
    exit 1
fi

# --- Release Logic ---
echo "================================================================================"
echo "Preparing release for ${ACTIVITY_NAME} with tag ${TAG}"
echo "================================================================================"

# Set environment variable for goreleaser
export ACTIVITY_NAME

# Create git tag locally
echo "Creating git tag: ${TAG}"
git tag "${TAG}"

# Run goreleaser. It will automatically pick up the tag, build both CP and CEDT binaries,
# and upload them as assets to the GitHub release.
# NOTE: You must have a GITHUB_TOKEN environment variable set for this to work.
echo "Running GoReleaser..."
goreleaser release --clean

# Push the new tag to the remote repository
echo "Pushing git tag to remote: ${TAG}"
git push origin "${TAG}"

echo "Release for ${TAG} completed successfully."