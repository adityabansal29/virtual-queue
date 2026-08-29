// queue.js — queue waiting page client
// D-02: all API calls go through window.QUEUE_CONFIG.apiBase (no hardcoded host)
const params   = new URLSearchParams(location.search);
const ticketId = params.get('ticket');
const target   = params.get('target');
const SSE_THRESHOLD = 200; // crossover point — matches server-side cfg.SSEThreshold

if (target) sessionStorage.setItem('q_target', target);

let pollTimer  = null;
let es         = null;
let navigating = false; // guard against duplicate handleAdmitted calls (UI-05)
let admitRatePerMin = 60; // 1/sec default; updated from poll response admitRate field

// UI-SPEC: loading state before first poll returns
document.getElementById('pos').textContent = 'Checking your position…';

function handleAdmitted(token) {
    if (navigating) return; // idempotent — second call is a no-op
    navigating = true;
    if (es) es.close();
    if (pollTimer) clearInterval(pollTimer);
    document.cookie = 'q_admission=' + token + '; path=/; max-age=1800; SameSite=Lax';
    window.location.href = sessionStorage.getItem('q_target') || '/';
}

function startSSE() {
    es = new EventSource(window.QUEUE_CONFIG.apiBase + '/queue/status/' + ticketId + '?mode=sse');
    es.addEventListener('update', function(e) {
        const data = JSON.parse(e.data);
        if (data.type === 'position') renderPosition(data.value, data);
        if (data.type === 'admitted') handleAdmitted(data.token);
        showConstrained(!!data.constrained);
    });
    // UI-SPEC copy: SSE error state — do NOT close es, EventSource reconnects itself
    es.onerror = function() {
        document.getElementById('status').textContent = 'Reconnecting…';
    };
}

async function pollOnce() {
    try {
        const res  = await fetch(window.QUEUE_CONFIG.apiBase + '/queue/status/' + ticketId + '?mode=poll');
        if (!res.ok) throw new Error('non-200');
        const data = await res.json();

        // update admitRatePerMin if server sends it
        if (data.admitRate) admitRatePerMin = data.admitRate * 60;

        if (data.type === 'admitted') { handleAdmitted(data.token); return; }
        if (data.type === 'pending') return; // try again next tick

        renderPosition(data.value, data);
        showConstrained(!!data.constrained);

        // crossover: close polling, switch to SSE for low-latency final stretch
        if (data.upgrade_to_sse && !es) {
            clearInterval(pollTimer);
            startSSE();
        }

        // clear error status on successful poll
        document.getElementById('status').textContent = '';
    } catch (_) {
        // UI-SPEC copy: poll error state
        document.getElementById('status').textContent = 'Connection lost. Retrying…';
    }
}

function renderPosition(rank, data) {
    // UI-SPEC copy contract
    document.getElementById('pos').textContent = rank + ' people ahead';
    const mins = Math.ceil(rank / admitRatePerMin);
    document.getElementById('wait').textContent =
        rank < 1 ? 'Less than a minute' : '~' + mins + ' min';
    showConstrained(!!(data && data.constrained));
}

function showConstrained(on) {
    // UI-SPEC: toggle CSS class 'visible' — CSS controls show/hide
    const banner = document.getElementById('constrained');
    const wait   = document.getElementById('wait');
    if (on) {
        banner.classList.add('visible');
        wait.style.display = 'none';
    } else {
        banner.classList.remove('visible');
        wait.style.display = '';
    }
}

if (!ticketId) {
    // Guard: no ticket param — show loading indefinitely, do not start polling
    document.getElementById('status').textContent = 'Checking your position…';
} else {
    // Start in polling mode; crossover to SSE happens automatically near the front
    pollTimer = setInterval(pollOnce, 5000);
    pollOnce(); // immediate first read, don't wait 5s
}
