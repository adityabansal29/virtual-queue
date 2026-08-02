#!/usr/bin/env bash
# verify.sh — Phase 1 success criteria verification
# Assumes docker compose stack is already running (use: make up && make verify)
set -euo pipefail

QSERVER="http://localhost:8080"
ORIGIN="http://localhost:8081"

EXIT_CODE=0

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

pass() { echo -e "${GREEN}[PASS]${NC} $1"; }
fail() { echo -e "${RED}[FAIL]${NC} $1"; EXIT_CODE=1; }

echo "=== Phase 1 Verification ==="
echo ""

# ---------------------------------------------------------------------------
# Criterion 1: stack reachable + POST /queue/join returns ticketId
# ---------------------------------------------------------------------------
echo "--- Criterion 1: Stack up + join ---"

HEALTH=$(curl -sf "${QSERVER}/health" 2>/dev/null || echo "")
if echo "$HEALTH" | grep -q '"ok":true'; then
  pass "Criterion 1a: queueserver health OK"
else
  fail "Criterion 1a: queueserver not reachable (got: ${HEALTH:-<empty>})"
fi

JOIN=$(curl -sf -X POST "${QSERVER}/queue/join" \
  -H "Content-Type: application/json" \
  -d '{"eventId":"evt-001"}' 2>/dev/null || echo "")
TICKET=$(echo "$JOIN" | grep -o '"ticketId":"[^"]*"' | cut -d'"' -f4)
if [ -n "$TICKET" ]; then
  pass "Criterion 1b: POST /queue/join returned ticketId=${TICKET}"
else
  fail "Criterion 1b: no ticketId in response (got: ${JOIN:-<empty>})"
fi

# ---------------------------------------------------------------------------
# Criterion 2: poll returns position and upgrade_to_sse hint
# ---------------------------------------------------------------------------
echo ""
echo "--- Criterion 2: Poll status ---"

# Join a second ticket; the first one may have been admitted already.
JOIN2=$(curl -sf -X POST "${QSERVER}/queue/join" \
  -H "Content-Type: application/json" \
  -d '{"eventId":"evt-001"}' 2>/dev/null || echo "")
TICKET2=$(echo "$JOIN2" | grep -o '"ticketId":"[^"]*"' | cut -d'"' -f4)

if [ -z "$TICKET2" ]; then
  fail "Criterion 2 setup: could not get TICKET2 for poll test"
else
  POLL=$(curl -sf "${QSERVER}/queue/status/${TICKET2}?mode=poll" 2>/dev/null || echo "")
  if echo "$POLL" | grep -q '"type":"position"'; then
    pass "Criterion 2a: poll returns position type"
  elif echo "$POLL" | grep -q '"type":"admitted"'; then
    pass "Criterion 2a: poll returned admitted (queue was empty)"
  else
    fail "Criterion 2a: poll did not return position or admitted (got: ${POLL:-<empty>})"
  fi

  if echo "$POLL" | grep -q '"upgrade_to_sse":true'; then
    pass "Criterion 2b: upgrade_to_sse:true (rank < 200)"
  elif echo "$POLL" | grep -q '"type":"admitted"'; then
    pass "Criterion 2b: skipped (already admitted — upgrade_to_sse not required)"
  else
    fail "Criterion 2b: upgrade_to_sse not true for rank < 200 (got: ${POLL:-<empty>})"
  fi
fi

# ---------------------------------------------------------------------------
# Criterion 3: SSE stream delivers position then admitted events
# ---------------------------------------------------------------------------
echo ""
echo "--- Criterion 3: SSE stream ---"

JOIN3=$(curl -sf -X POST "${QSERVER}/queue/join" \
  -H "Content-Type: application/json" \
  -d '{"eventId":"evt-001"}' 2>/dev/null || echo "")
TICKET3=$(echo "$JOIN3" | grep -o '"ticketId":"[^"]*"' | cut -d'"' -f4)

if [ -z "$TICKET3" ]; then
  fail "Criterion 3 setup: could not get TICKET3 for SSE test"
else
  # --max-time 5: avoid hanging indefinitely; scheduler admits within 1s.
  SSE_OUTPUT=$(curl -sf -N --max-time 5 \
    "${QSERVER}/queue/status/${TICKET3}?mode=sse" 2>/dev/null || echo "")

  if echo "$SSE_OUTPUT" | grep -q '"type":"position"'; then
    pass "Criterion 3a: SSE delivered position event"
  else
    fail "Criterion 3a: SSE did not deliver position event (got: ${SSE_OUTPUT:-<empty>})"
  fi

  if echo "$SSE_OUTPUT" | grep -q '"type":"admitted"'; then
    pass "Criterion 3b: SSE delivered admitted event"
  else
    fail "Criterion 3b: SSE did not deliver admitted event (timeout or scheduler not running?)"
  fi
fi

# ---------------------------------------------------------------------------
# Criterion 4: QueueGuard one-time enforcement (SETNX)
# ---------------------------------------------------------------------------
echo ""
echo "--- Criterion 4: QueueGuard one-time enforcement ---"

JOIN4=$(curl -sf -X POST "${QSERVER}/queue/join" \
  -H "Content-Type: application/json" \
  -d '{"eventId":"evt-001"}' 2>/dev/null || echo "")
TICKET4=$(echo "$JOIN4" | grep -o '"ticketId":"[^"]*"' | cut -d'"' -f4)

if [ -z "$TICKET4" ]; then
  fail "Criterion 4 setup: could not get TICKET4"
else
  # Poll until admitted — max 8 attempts (scheduler tick is 1s)
  JWT=""
  for i in $(seq 1 8); do
    POLL4=$(curl -sf "${QSERVER}/queue/status/${TICKET4}?mode=poll" 2>/dev/null || echo "")
    JWT=$(echo "$POLL4" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
    [ -n "$JWT" ] && break
    sleep 1
  done

  if [ -z "$JWT" ]; then
    fail "Criterion 4: could not obtain admission JWT within 8s"
  else
    # First use — must succeed (200)
    HTTP1=$(curl -sf -o /dev/null -w "%{http_code}" \
      -b "q_admission=${JWT}" "${ORIGIN}/" 2>/dev/null || echo "000")
    if [ "$HTTP1" = "200" ]; then
      pass "Criterion 4a: first admission HTTP 200"
    else
      fail "Criterion 4a: first admission returned HTTP ${HTTP1} (expected 200)"
    fi

    # Second use (replay) — must be rejected (403)
    HTTP2=$(curl -sf -o /dev/null -w "%{http_code}" \
      -b "q_admission=${JWT}" "${ORIGIN}/" 2>/dev/null || echo "000")
    if [ "$HTTP2" = "403" ]; then
      pass "Criterion 4b: token replay returned HTTP 403"
    else
      fail "Criterion 4b: token replay returned HTTP ${HTTP2} (expected 403)"
    fi

    # Redis introspection: token:{jti} must exist in redis-origin (confirms SETNX)
    # Decode JWT payload — handle both macOS (no --ignore-garbage) and GNU base64.
    JWT_PAYLOAD=$(echo "$JWT" | cut -d. -f2)
    # Pad to multiple of 4 for base64 decode
    PAD=$(( 4 - ${#JWT_PAYLOAD} % 4 ))
    [ "$PAD" -ne 4 ] && JWT_PAYLOAD="${JWT_PAYLOAD}$(printf '%0.s=' $(seq 1 $PAD))"
    JTI=$(echo "$JWT_PAYLOAD" | base64 -d 2>/dev/null | python3 -c \
      "import sys,json; d=json.load(sys.stdin); print(d.get('jti',''))" 2>/dev/null || echo "")

    if [ -n "$JTI" ]; then
      EXISTS=$(docker compose exec -T redis-origin redis-cli EXISTS "token:${JTI}" 2>/dev/null || echo "0")
      if echo "$EXISTS" | grep -q "1"; then
        pass "Criterion 4c: token:${JTI} exists in redis-origin (SETNX confirmed)"
      else
        fail "Criterion 4c: token:{jti} not found in redis-origin (EXISTS returned: ${EXISTS})"
      fi
    else
      fail "Criterion 4c: could not extract JTI from JWT for redis introspection"
    fi
  fi
fi

# ---------------------------------------------------------------------------
# Criterion 5: env vars — two distinct secrets, wrong-secret rejection
# ---------------------------------------------------------------------------
echo ""
echo "--- Criterion 5: Secret isolation ---"

if [ -f ".env.example" ]; then
  ADM=$(grep "^ADMISSION_SECRET=" .env.example | cut -d= -f2)
  SES=$(grep "^SESSION_SECRET=" .env.example | cut -d= -f2)
  if [ -n "$ADM" ] && [ -n "$SES" ] && [ "$ADM" != "$SES" ]; then
    pass "Criterion 5a: ADMISSION_SECRET and SESSION_SECRET are distinct non-empty values in .env.example"
  else
    fail "Criterion 5a: secrets missing or identical in .env.example (ADM=${ADM:-<empty>}, SES=${SES:-<empty>})"
  fi
else
  fail "Criterion 5a: .env.example not found"
fi

# Present a JWT signed with a fake secret — must NOT return 200
FAKE_JWT="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ0ZXN0IiwiZXhwIjo5OTk5OTk5OTk5fQ.FAKESIGNATURE"
HTTP_FAKE=$(curl -sf -o /dev/null -w "%{http_code}" \
  -b "q_admission=${FAKE_JWT}" "${ORIGIN}/" 2>/dev/null || echo "000")
if [ "$HTTP_FAKE" != "200" ]; then
  pass "Criterion 5b: wrong-secret JWT rejected (HTTP ${HTTP_FAKE}, not 200)"
else
  fail "Criterion 5b: wrong-secret JWT was accepted (should be rejected)"
fi

# ---------------------------------------------------------------------------
# Final result
# ---------------------------------------------------------------------------
echo ""
if [ "$EXIT_CODE" = "0" ]; then
  echo "All criteria PASSED"
else
  echo "Some criteria FAILED — see output above"
fi
exit "$EXIT_CODE"
