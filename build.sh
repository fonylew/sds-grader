#!/bin/sh

# --- Configuration ---
# This script builds a specific activity for multiple platforms and for all topic variants (CP, CEDT).
#
# Usage: ./build.sh <activity_name>
#
# Example: ./build.sh activity1
#
# <activity_name>: The directory name of the activity (e.g., activity1).

# --- Input Validation ---
if [ -z "$1" ]; then
    echo "Error: Activity name is required."
    echo "Usage: $0 <activity_name>"
    exit 1
fi

ACTIVITY_NAME=$1

if [ ! -d "$ACTIVITY_NAME" ]; then
    echo "Error: Activity directory './${ACTIVITY_NAME}' not found."
    exit 1
fi

# --- Build Logic ---
VAR_PATH="main.topic"
TOPIC_SUFFIXES=("CP" "CEDT")

mkdir -p $ACTIVITY_NAME/bin

for TOPIC_SUFFIX in "${TOPIC_SUFFIXES[@]}"; do
    TOPIC_NAME="${ACTIVITY_NAME}_${TOPIC_SUFFIX}"

    echo "--------------------------------------------------"
    echo "Building activity: ${ACTIVITY_NAME} for ${TOPIC_SUFFIX}"
    echo "Injecting topic: ${TOPIC_NAME}"
    echo "--------------------------------------------------"

# Build for different operating systems and architectures
    GOOS=darwin GOARCH=arm64 go build -ldflags "-X ${VAR_PATH}=${TOPIC_NAME}" -o "${ACTIVITY_NAME}/bin/${ACTIVITY_NAME}-macos-arm64-${TOPIC_SUFFIX}" "./${ACTIVITY_NAME}/"
    GOOS=darwin GOARCH=amd64 go build -ldflags "-X ${VAR_PATH}=${TOPIC_NAME}" -o "${ACTIVITY_NAME}/bin/${ACTIVITY_NAME}-macos-intel-${TOPIC_SUFFIX}" "./${ACTIVITY_NAME}/"
    GOOS=linux GOARCH=amd64 go build -ldflags "-X ${VAR_PATH}=${TOPIC_NAME}" -o "${ACTIVITY_NAME}/bin/${ACTIVITY_NAME}-linux-${TOPIC_SUFFIX}" "./${ACTIVITY_NAME}/"
    GOOS=windows GOARCH=amd64 go build -ldflags "-X ${VAR_PATH}=${TOPIC_NAME}" -o "${ACTIVITY_NAME}/bin/${ACTIVITY_NAME}-windows-${TOPIC_SUFFIX}.exe" "./${ACTIVITY_NAME}/"

# Make the binaries executable
    chmod +x "${ACTIVITY_NAME}/bin/${ACTIVITY_NAME}-macos-arm64-${TOPIC_SUFFIX}" 2>/dev/null || true
    chmod +x "${ACTIVITY_NAME}/bin/${ACTIVITY_NAME}-macos-intel-${TOPIC_SUFFIX}" 2>/dev/null || true
    chmod +x "${ACTIVITY_NAME}/bin/${ACTIVITY_NAME}-linux-${TOPIC_SUFFIX}" 2>/dev/null || true
done

echo ""
echo "Build complete. All binaries are in the 'bin' directory."
