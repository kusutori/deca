#!/usr/bin/env bash
set -euo pipefail

COVERAGE_FILE="${1:-coverage.out}"
BASELINE_COVERAGE="${BASELINE_COVERAGE:-}"
STAGE="${COVERAGE_STAGE:-final}"

if [[ ! -f "$COVERAGE_FILE" ]]; then
  echo "coverage file not found: $COVERAGE_FILE" >&2
  exit 1
fi

threshold=""
case "$STAGE" in
  stage1)
    threshold="60"
    ;;
  stage2)
    threshold="70"
    ;;
  final)
    threshold="80"
    ;;
  *)
    echo "invalid COVERAGE_STAGE: $STAGE (expected: stage1|stage2|final)" >&2
    exit 1
    ;;
esac

total=$(go tool cover -func="$COVERAGE_FILE" | awk '/^total:/ {print $3}' | tr -d '%')
if [[ -z "$total" ]]; then
  echo "failed to parse total coverage from $COVERAGE_FILE" >&2
  exit 1
fi

echo "Coverage stage: $STAGE"
echo "Total coverage: ${total}%"
echo "Threshold: ${threshold}%"

awk -v total="$total" -v min="$threshold" 'BEGIN { if (total+0 < min+0) { printf("Coverage %.2f%% is below %.2f%%\n", total, min); exit 1 } }'

# Per-package stricter thresholds.
config_cov=$(go test ./internal/config -cover | awk '/^coverage:/ {print $2}' | tr -d '%')
install_cov=$(go test ./internal/install -cover | awk '/^coverage:/ {print $2}' | tr -d '%')

echo "internal/config coverage: ${config_cov}% (threshold: 85%)"
echo "internal/install coverage: ${install_cov}% (threshold: 85%)"

awk -v total="$config_cov" 'BEGIN { if (total+0 < 85) { printf("internal/config coverage %.2f%% is below 85%%\n", total); exit 1 } }'
awk -v total="$install_cov" 'BEGIN { if (total+0 < 85) { printf("internal/install coverage %.2f%% is below 85%%\n", total); exit 1 } }'

if [[ -n "$BASELINE_COVERAGE" ]]; then
  echo "Baseline coverage: ${BASELINE_COVERAGE}%"
  awk -v head="$total" -v base="$BASELINE_COVERAGE" 'BEGIN { if (head+0 < base+0) { printf("Coverage regression: %.2f%% < baseline %.2f%%\n", head, base); exit 1 } }'
fi
