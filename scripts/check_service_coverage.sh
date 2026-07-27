#!/usr/bin/env bash
set -e

# Threshold percentage for service layer code coverage
THRESHOLD=75.0

echo "🔍 Running unit tests and measuring coverage for service layer (./internal/service/...)..."

# Run tests with race detection and generate coverage profile
go test -race -coverprofile=coverage.out ./internal/service/...

# Extract raw percentage string (e.g., 78.2%)
COVERAGE_STR=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | tr -d '%')

if [ -z "$COVERAGE_STR" ]; then
    echo "❌ Error: Could not determine code coverage percentage."
    exit 1
fi

echo "📊 Current Service Layer Coverage: ${COVERAGE_STR}%"
echo "🎯 Minimum Required Threshold: ${THRESHOLD}%"

# Perform floating point comparison using awk
IS_SUFFICIENT=$(awk -v cov="$COVERAGE_STR" -v thresh="$THRESHOLD" 'BEGIN { print (cov >= thresh) ? "1" : "0" }')

if [ "$IS_SUFFICIENT" -eq 1 ]; then
    echo "✅ SUCCESS: Service layer test coverage standard met (${COVERAGE_STR}% >= ${THRESHOLD}%)!"
    exit 0
else
    echo "❌ FAILURE: Service layer test coverage is below requirement (${COVERAGE_STR}% < ${THRESHOLD}%)."
    echo "👉 Please write additional unit tests under ./internal/service/... to reach at least ${THRESHOLD}% coverage before opening/merging a Pull Request."
    exit 1
fi
