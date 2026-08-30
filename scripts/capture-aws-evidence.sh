#!/usr/bin/env bash
# Capture deployed AWS queue evidence: 20 seed users, browser poll -> SSE, and ECS logs.
set -euo pipefail

API_BASE="${QUEUE_API_BASE:-https://d20ahx254u6i0j.cloudfront.net}"
PAGE_BASE="${QUEUE_PAGE_BASE:-https://d3ef9uo0iwz4l3.cloudfront.net/queue/index.html}"
EVENT_ID="${EVENT_ID:-evidence-$(date -u +%Y%m%d%H%M%S)}"
COUNT="${QUEUE_SEED_COUNT:-20}"
OUT_DIR="${EVIDENCE_DIR:-test-screenshots}"
PROGRESS_INTERVAL_SECS="${PROGRESS_INTERVAL_SECS:-5}"
PROGRESS_STOP_RANK="${PROGRESS_STOP_RANK:-0}"
mkdir -p "$OUT_DIR"
CAPTURE_START_MS="$(( $(date +%s) * 1000 ))"

echo "Seeding ${COUNT} queue tickets for ${EVENT_ID}..."
seq 1 "$COUNT" | xargs -P "$COUNT" -I _ curl -fsS -o /dev/null "${API_BASE}/queue/join?eventId=${EVENT_ID}"

NODE_PATH_VALUE="${PLAYWRIGHT_NODE_PATH:-}"
if ! NODE_PATH="$NODE_PATH_VALUE" node -e "require('playwright-core')" >/dev/null 2>&1; then
  PLAYWRIGHT_DIR="$(mktemp -d /tmp/virtual-queue-playwright.XXXXXX)"
  npm install --silent --prefix "$PLAYWRIGHT_DIR" --no-save playwright-core@1.62.1
  NODE_PATH_VALUE="$PLAYWRIGHT_DIR/node_modules"
fi

export API_BASE PAGE_BASE EVENT_ID OUT_DIR PROGRESS_INTERVAL_SECS PROGRESS_STOP_RANK
NODE_PATH="$NODE_PATH_VALUE" node <<'NODE'
const fs = require('fs');
const { chromium } = require('playwright-core');

const api = process.env.API_BASE;
const pageBase = process.env.PAGE_BASE;
const eventId = encodeURIComponent(process.env.EVENT_ID);
const out = process.env.OUT_DIR;
const chrome = process.env.CHROME_PATH || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const events = [];
const consoleLines = [];
const errors = [];

(async () => {
  const browser = await chromium.launch({ headless: true, executablePath: chrome });
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  page.on('console', msg => consoleLines.push(`[${msg.type()}] ${msg.text()}`));
  page.on('pageerror', err => errors.push(`[pageerror] ${err.message}`));
  page.on('request', req => {
    const url = req.url();
    if (url.includes('/queue/status/')) events.push(`[${new Date().toISOString()}] request ${new URL(url).searchParams.get('mode')}`);
  });
  page.on('response', res => {
    const url = res.url();
    if (url.includes('/queue/status/')) events.push(`[${new Date().toISOString()}] response ${res.status()} ${new URL(url).searchParams.get('mode')}`);
    if (res.status() >= 400) consoleLines.push(`[http ${res.status()}] ${url}`);
  });

  const join = await page.goto(`${api}/queue/join?eventId=${eventId}&target=${encodeURIComponent(pageBase + '?complete=1')}`, { waitUntil: 'domcontentloaded', timeout: 30000 });
  if (!join || join.status() !== 200) throw new Error(`join navigation failed: ${join && join.status()}`);
  await page.waitForFunction(() => document.querySelector('#pos')?.textContent?.includes('people ahead'), null, { timeout: 10000 });
  await page.screenshot({ path: `${out}/aws-queue-polling.png`, fullPage: true });
  const pollingText = await page.locator('body').innerText();
  await page.waitForTimeout(1500);
  await page.screenshot({ path: `${out}/aws-queue-sse.png`, fullPage: true });
  const sseText = await page.locator('body').innerText();
  const progress = [];
  let lastRank = null;
  const interval = Number(process.env.PROGRESS_INTERVAL_SECS) * 1000;
  const stopRank = Number(process.env.PROGRESS_STOP_RANK);
  for (;;) {
    const text = await page.locator('#pos').innerText();
    const match = text.match(/^(\d+) people ahead$/);
    if (!match) break;
    const rank = Number(match[1]);
    if (rank !== lastRank) {
      const filename = `${out}/aws-queue-progress-${rank}.png`;
      await page.screenshot({ path: filename, fullPage: true });
      progress.push(JSON.stringify({
        iteration: progress.length + 1,
        timestamp: new Date().toISOString(),
        rank,
        ui: text,
        screenshot: filename,
      }));
      lastRank = rank;
    }
    if (rank <= stopRank) break;
    await page.waitForTimeout(interval);
  }
  const finalUrl = page.url();
  await browser.close();

  fs.writeFileSync(`${out}/aws-queue-browser.log`, [
    'Browser network events:', ...events,
    '', 'JavaScript console:', ...(consoleLines.length ? consoleLines : ['<no console messages>']),
    '', 'JavaScript page errors:', ...(errors.length ? errors : ['<no page errors>']),
    '', `Polling UI: ${JSON.stringify(pollingText)}`,
    `SSE UI: ${JSON.stringify(sseText)}`,
    '', 'Queue progression (one record per rank):', ...(progress.length ? progress : ['<none>']),
    `Final URL: ${finalUrl}`,
  ].join('\n') + '\n');
})().catch(err => { console.error(err.stack || err); process.exit(1); });
NODE

# Wait for the configured 20-second scheduler tick so the run also has an
# admission record in CloudWatch. Override for a different SCHEDULER_TICK_SECS.
sleep "${LOG_WAIT_SECS:-22}"
CAPTURE_END_MS="$(( $(date +%s) * 1000 ))"

{
  echo "AWS queue evidence $(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  echo
  echo "== Browser JavaScript/network =="
  sed -n '1,160p' "$OUT_DIR/aws-queue-browser.log"
  echo
  echo "== QueueServer CloudWatch (/ecs/dev/queueserver) =="
  aws logs filter-log-events --log-group-name /ecs/dev/queueserver \
    --start-time "$CAPTURE_START_MS" --end-time "$CAPTURE_END_MS" \
    --query 'events[].message' --output text \
    | tr '\t' '\n' | rg 'queue/(join|status)' | sed -E 's/(token|jwt|authorization)[=:][^ ,]+/\1=<redacted>/Ig' || true
  echo
  echo "== Scheduler CloudWatch (/ecs/dev/scheduler) =="
  aws logs filter-log-events --log-group-name /ecs/dev/scheduler \
    --start-time "$CAPTURE_START_MS" --end-time "$CAPTURE_END_MS" \
    --query 'events[].message' --output text \
    | tr '\t' '\n' | rg 'admitted batch' | sed -E 's/(token|jwt|authorization)[=:][^ ,]+/\1=<redacted>/Ig' || true
} > "$OUT_DIR/aws-queue-evidence.txt"

echo "Evidence written to $OUT_DIR/aws-queue-evidence.txt"
