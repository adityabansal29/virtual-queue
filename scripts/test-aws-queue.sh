#!/usr/bin/env bash
# AWS smoke test: join 20 independent users, then verify user 20 via poll -> SSE.
set -euo pipefail

API_BASE="${QUEUE_API_BASE:?Set QUEUE_API_BASE to the dev or prod queue API URL}"
COUNT="${QUEUE_TEST_COUNT:-20}"
EVENT_ID="${EVENT_ID:-aws-smoke-$(date -u +%Y%m%d%H%M%S)}"
WAIT_SECS="${WAIT_SECS:-180}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/virtual-queue-aws.XXXXXX")"
trap 'rm -rf "$WORK_DIR"' EXIT

join_user() {
  local user="$1" headers="$WORK_DIR/headers-$1" cookie="$WORK_DIR/cookie-$1" ticket_file="$WORK_DIR/ticket-$1"
  curl -fsS --retry 3 --connect-timeout 10 \
    -D "$headers" -o /dev/null -c "$cookie" \
    "$API_BASE/queue/join?eventId=$EVENT_ID"
  awk 'tolower($1) == "location:" {print $2}' "$headers" | tr -d '\r' \
    | sed -n 's/.*[?&]ticket=\([^&]*\).*/\1/p' > "$ticket_file"
  test -s "$ticket_file" || { echo "user $user: join did not return a ticket" >&2; return 1; }
}

echo "joining $COUNT independent users to event $EVENT_ID"
for user in $(seq -w 1 "$COUNT"); do
  join_user "$user"
done

LAST_USER="$(printf '%0*d' "${#COUNT}" "$COUNT")"
TICKET="$(cat "$WORK_DIR/ticket-$LAST_USER")"
COOKIE="$WORK_DIR/cookie-$LAST_USER"
STATUS_URL="$API_BASE/queue/status/$TICKET"

echo "user $COUNT joined with an isolated ticket"
DEADLINE=$(( $(date +%s) + WAIT_SECS ))
while :; do
  POLL_FILE="$WORK_DIR/poll"
  curl -fsS --cookie "$COOKIE" "$STATUS_URL?mode=poll" -o "$POLL_FILE"
  POLL_STATE="$(python3 - "$POLL_FILE" <<'PY'
import json, sys
data = json.load(open(sys.argv[1]))
print(data.get("type", "") + "|" + str(data.get("upgrade_to_sse", False)).lower())
if data.get("type") not in {"position", "admitted"}:
    raise SystemExit("poll did not return position/admitted")
PY
  )"
  IFS='|' read -r POLL_TYPE UPGRADE_TO_SSE <<< "$POLL_STATE"
  echo "20th user poll: type=$POLL_TYPE upgrade_to_sse=$UPGRADE_TO_SSE"
  if [[ "$POLL_TYPE" == "admitted" ]]; then
    echo "AWS queue smoke test passed: $COUNT joins + authenticated 20th-user poll"
    exit 0
  fi
  [[ "$UPGRADE_TO_SSE" == "true" ]] && break
  (( $(date +%s) < DEADLINE )) || { echo "20th user did not reach SSE tier" >&2; exit 1; }
  sleep 5
done

SSE_FILE="$WORK_DIR/sse"
echo "opening authenticated SSE stream for user $COUNT (up to ${WAIT_SECS}s)"
curl -fsS -N --max-time "$WAIT_SECS" --cookie "$COOKIE" \
  "$STATUS_URL?mode=sse" -o "$SSE_FILE" || true

if grep -q '"type":"position"' "$SSE_FILE"; then
  echo "20th user SSE: position event received"
fi
if grep -q '"type":"admitted"' "$SSE_FILE"; then
  echo "20th user SSE: admitted event received"
else
  echo "20th user SSE: no admission within ${WAIT_SECS}s (increase WAIT_SECS if expected)" >&2
  exit 1
fi

echo "AWS queue smoke test passed: $COUNT joins + authenticated 20th-user poll/SSE"
