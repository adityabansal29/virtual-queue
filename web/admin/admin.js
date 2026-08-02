/* admin.js — live queue admin dashboard */
let pollTimer = null;
let currentEventId = null;
let lastStats = null;

async function loadEvents() {
    try {
        const res = await fetch(`${window.QUEUE_CONFIG.apiBase}/queue/events`);
        const { events } = await res.json();
        const sel = document.getElementById('event-select');
        // clear existing options except placeholder
        sel.innerHTML = '<option value="">No events active</option>';
        if (!events || !events.length) {
            setStatsBlank();
            document.getElementById('update-btn').disabled = true;
            return;
        }
        events.forEach(id => {
            const opt = document.createElement('option');
            opt.value = id;
            opt.textContent = id;
            sel.appendChild(opt);
        });
        startPolling(events[0]);
        sel.value = events[0];
    } catch (_) {
        setStatsBlank();
    }
}

function startPolling(eventId) {
    if (pollTimer) clearInterval(pollTimer);
    currentEventId = eventId;
    document.getElementById('update-btn').disabled = false;
    setPollIndicator(true);
    fetchConfig();
    pollTimer = setInterval(fetchConfig, 2000);
}

async function fetchConfig() {
    try {
        const res = await fetch(`${window.QUEUE_CONFIG.apiBase}/queue/config/${currentEventId}`);
        if (!res.ok) throw new Error('non-200');
        const data = await res.json();
        lastStats = data;
        renderStats(data, false);
    } catch (_) {
        // UI-SPEC: retain last value with "(stale)" suffix
        if (lastStats) renderStats(lastStats, true);
    }
}

function renderStats(data, stale) {
    const s = stale ? ' (stale)' : '';
    document.getElementById('stat-depth').textContent    = data.queueDepth + s;
    document.getElementById('stat-active').textContent   = data.activeUsers + s;
    document.getElementById('stat-rate').textContent     = data.admitRate + s;
    document.getElementById('stat-capacity').textContent = data.capacity + s;

    // headroom computed client-side (D-08); accent when < 10% of capacity
    const headroomEl = document.getElementById('stat-headroom');
    if (data.capacity > 0) {
        const headroom = data.capacity - data.activeUsers;
        headroomEl.textContent = headroom + s;
        headroomEl.classList.toggle('accent', !stale && headroom < data.capacity * 0.1);
    } else {
        headroomEl.textContent = '—'; // "—"
        headroomEl.classList.remove('accent');
    }

    // drain: negative estimatedDrainSec means rate=0, show "—"
    const drainEl = document.getElementById('stat-drain');
    drainEl.textContent = (data.estimatedDrainSec >= 0)
        ? Math.ceil(data.estimatedDrainSec / 60) + ' min' + s
        : '—';
}

function setStatsBlank() {
    ['stat-depth', 'stat-active', 'stat-rate', 'stat-capacity', 'stat-headroom', 'stat-drain']
        .forEach(id => { document.getElementById(id).textContent = '—'; });
}

function setPollIndicator(active) {
    const el = document.getElementById('poll-indicator');
    if (active) {
        el.classList.add('active');
    } else {
        el.classList.remove('active');
    }
}

document.getElementById('event-select').addEventListener('change', function (e) {
    if (e.target.value) startPolling(e.target.value);
});

document.getElementById('update-form').addEventListener('submit', async function (e) {
    e.preventDefault();
    if (!currentEventId) return;
    const btn = document.getElementById('update-btn');
    const errEl = document.getElementById('update-error');
    const rate = parseInt(document.getElementById('rate-input').value, 10);
    const capacity = parseInt(document.getElementById('capacity-input').value, 10);
    if (isNaN(rate) || rate < 1) return;
    btn.disabled = true;
    errEl.textContent = '';
    try {
        const res = await fetch(`${window.QUEUE_CONFIG.apiBase}/queue/rate/${currentEventId}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ rate, capacity: isNaN(capacity) ? 0 : capacity }),
        });
        if (!res.ok) throw new Error('non-200');
        btn.textContent = 'Saved';
        btn.disabled = false;
        setTimeout(function () { btn.textContent = 'Update Config'; }, 1500);
    } catch (_) {
        // UI-SPEC copy: "Update failed. Try again."
        errEl.textContent = 'Update failed. Try again.';
        btn.disabled = false;
    }
});

document.addEventListener('DOMContentLoaded', loadEvents);
