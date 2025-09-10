#!/bin/bash
set -e # Exit immediately if a command exits with a non-zero status.

# --- Configuration ---
# This script creates and pushes a release for a specific activity and major using goreleaser.
#
# Usage: ./release.sh <activity_name> <version>
#
# Example: ./release.sh activity1 v0.1.0
#
# This will create and publish two GitHub releases with tags:
# - v0.1.0-activity1-CP
# - v0.1.0-activity1-CEDT

# --- Input Validation ---
if [ -z "$1" ] || [ -z "$2" ]; then
    echo "Error: Activity name and version are required."
    echo "Usage: $0 <activity_name> <version>"
    echo "Example: $0 activity1 v0.1.0"
    exit 1
fi

ACTIVITY_NAME=$1
VERSION=$2
TOPIC_SUFFIXES=("CP" "CEDT")

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
for TOPIC_SUFFIX in "${TOPIC_SUFFIXES[@]}"; do
    TAG="${VERSION}-${ACTIVITY_NAME}-${TOPIC_SUFFIX}"

    echo "================================================================================"
    echo "Preparing release for ${ACTIVITY_NAME} (${TOPIC_SUFFIX}) with tag ${TAG}"
    echo "================================================================================"

    # Set environment variables for goreleaser to use
    export ACTIVITY_NAME
    export TOPIC_SUFFIX

    # Create git tag locally
    echo "Creating git tag: ${TAG}"
    git tag "${TAG}"

    # Run goreleaser. It will automatically pick up the latest tag, create a release,
    # build binaries, and upload them as assets to the GitHub release.
    # NOTE: You must have a GITHUB_TOKEN environment variable set for this to work.
    echo "Running GoReleaser..."
    goreleaser release --clean

#    # Push the new tag to the remote repository
#    echo "Pushing git tag to remote: ${TAG}"
#    git push origin "${TAG}"

    echo "Release for ${TAG} completed successfully."
done

echo ""
echo "All releases have been processed."