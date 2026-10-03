#!/usr/bin/env bash
# Create/update all application-owned SSM parameters for one environment.
set -euo pipefail

ENVIRONMENT="${1:?usage: $0 <dev|prod>}"
AWS_REGION="${AWS_REGION:-ap-south-1}"
PREFIX="/virtual-queue/$ENVIRONMENT"

: "${ADMISSION_SECRET:?set ADMISSION_SECRET}"
: "${SESSION_SECRET:?set SESSION_SECRET}"
: "${INTERNAL_API_TOKEN:?set INTERNAL_API_TOKEN}"
: "${DEFAULT_ADMIT_RATE:?set DEFAULT_ADMIT_RATE}"
: "${SSE_THRESHOLD:?set SSE_THRESHOLD}"
: "${SCHEDULER_TICK_SECS:?set SCHEDULER_TICK_SECS}"
: "${QUEUE_JOIN_URL:?set QUEUE_JOIN_URL}"
: "${QUEUE_VALIDATION_URL:?set QUEUE_VALIDATION_URL}"

[[ "$ADMISSION_SECRET" != "$SESSION_SECRET" ]] || {
  echo "ADMISSION_SECRET and SESSION_SECRET must differ" >&2
  exit 1
}

put() {
  local name="$1" type="$2" value="$3"
  aws ssm put-parameter \
    --name "$PREFIX/$name" \
    --type "$type" \
    --value "$value" \
    --overwrite \
    --region "$AWS_REGION" >/dev/null
}

put ADMISSION_SECRET SecureString "$ADMISSION_SECRET"
put SESSION_SECRET SecureString "$SESSION_SECRET"
put INTERNAL_API_TOKEN SecureString "$INTERNAL_API_TOKEN"
put DEFAULT_ADMIT_RATE String "$DEFAULT_ADMIT_RATE"
put SSE_THRESHOLD String "$SSE_THRESHOLD"
put SCHEDULER_TICK_SECS String "$SCHEDULER_TICK_SECS"
put QUEUE_JOIN_URL String "$QUEUE_JOIN_URL"
put QUEUE_VALIDATION_URL String "$QUEUE_VALIDATION_URL"

echo "Updated eight SSM parameters under $PREFIX"
