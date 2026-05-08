'use strict';

// Backend: dashai-api Python simulator
const API_BASE = 'https://dashai-api.onrender.com/factory/api/v1/equipment/studio';

let lastTraceId = 0;
let validationLog = [];

// --- REST API ---
async function apiGet(path) {
  try {
    const resp = await fetch(API_BASE + path);
    const data = await resp.json();
    return data.success ? data.data : null;
  } catch (e) {
    setConnectionStatus(false);
    return null;
  }
}

async function apiPost(path, body) {
  try {
    const resp = await fetch(API_BASE + path, {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(body),
    });
    const data = await resp.json();
    return data.success ? data.data : null;
  } catch (e) {
    return null;
  }
}

// --- Polling (3s) ---
async function pollTrace() {
  const traces = await apiGet('/trace');
  if (!traces) { setConnectionStatus(false); return; }
  setConnectionStatus(true);
  for (const entry of traces) {
    if (entry.id > lastTraceId) {
      lastTraceId = entry.id;
      appendTrace(entry);
    }
  }
}

function startPolling() {
  pollTrace();
  setInterval(pollTrace, 3000);
}

function setConnectionStatus(connected) {
  const el = document.getElementById('conn-status');
  const text = document.getElementById('conn-text');
  el.className = 'conn-status ' + (connected ? 'connected' : 'disconnected');
  const tt = (typeof t === 'function') ? t : (k => k);
  text.textContent = tt(connected ? 'conn.connected' : 'conn.disconnected');
}

// --- Trace ---
function appendTrace(entry) {
  const feed = document.getElementById('live-feed');
  if (!feed) return;
  if (feed.querySelector('.empty-state')) feed.innerHTML = '';

  const ts = new Date(entry.timestamp).toLocaleTimeString('en-US', {hour12:false, fractionalSecondDigits:1});
  const sf = 'S' + entry.stream + 'F' + entry.function;
  const dirClass = entry.direction === 'tx' ? 'tx' : 'rx';

  let badgeClass = 'pass', badgeText = 'PASS';
  if (entry.validation) {
    for (const v of entry.validation) {
      if (v.level === 2) { badgeClass = 'fail'; badgeText = 'FAIL'; break; }
      if (v.level === 1) { badgeClass = 'warn'; badgeText = 'WARN'; }
    }
    for (const v of entry.validation) validationLog.push({sf, ...v});
    renderValidation();
  }

  const item = document.createElement('div');
  item.className = 'feed-item';
  item.innerHTML = '<span class="feed-time">' + ts + '</span>'
    + '<span class="feed-dir ' + dirClass + '">' + entry.direction.toUpperCase() + '</span>'
    + '<span class="feed-sf">' + sf + '</span>'
    + '<span class="feed-desc">' + (entry.bodySml || '(empty)').substring(0, 60) + '</span>'
    + '<span class="feed-badge ' + badgeClass + '">' + badgeText + '</span>';
  feed.appendChild(item);
  feed.scrollTop = feed.scrollHeight;

  const sim = document.getElementById('sim-feed');
  if (sim) {
    if (sim.querySelector('.empty-state')) sim.innerHTML = '';
    const bubble = document.createElement('div');
    bubble.className = 'sim-msg ' + dirClass;
    bubble.innerHTML = '<div class="sim-msg-meta"><span>' + entry.direction.toUpperCase() + ' ' + sf + '</span><span class="sim-msg-time">' + ts + '</span></div>'
      + '<div class="sim-msg-body">' + (entry.bodySml || '(empty)') + '</div>';
    sim.prepend(bubble);
    sim.scrollTop = 0;
  }

  const c = document.getElementById('msg-count');
  if (c) c.textContent = lastTraceId;
  const v = document.getElementById('val-count');
  if (v) v.textContent = validationLog.length;
}

// --- Validator ---
function renderValidation() {
  const container = document.getElementById('validation-list');
  if (!container) return;

  const stats = {pass: 0, warn: 0, fail: 0};
  const dedup = new Map();
  for (const v of validationLog) {
    const cls = ['pass','warn','fail'][v.level] || 'pass';
    stats[cls]++;
    const key = (v.sf||'') + '|' + cls + '|' + (v.message||'');
    if (dedup.has(key)) dedup.get(key).count++;
    else dedup.set(key, {sf: v.sf, level: v.level, message: v.message, count: 1});
  }

  const total = stats.pass + stats.warn + stats.fail;
  const tt = (typeof t === 'function') ? t : (k => k);

  let html = '<div class="val-stat-grid">'
    + '<div class="val-stat pass"><div class="val-stat-num">' + stats.pass + '</div><div class="val-stat-label">' + tt('val.stat.pass') + '</div></div>'
    + '<div class="val-stat warn"><div class="val-stat-num">' + stats.warn + '</div><div class="val-stat-label">' + tt('val.stat.warn') + '</div></div>'
    + '<div class="val-stat fail"><div class="val-stat-num">' + stats.fail + '</div><div class="val-stat-label">' + tt('val.stat.fail') + '</div></div>'
    + '<div class="val-stat total"><div class="val-stat-num">' + total + '</div><div class="val-stat-label">' + tt('val.stat.total') + '</div></div>'
    + '</div>';

  if (dedup.size === 0) {
    html += '<div class="empty-state">' + tt('val.empty') + '</div>';
  } else {
    const sorted = [...dedup.values()].sort((a, b) => b.count - a.count);
    for (const v of sorted) {
      const cls = ['pass','warn','fail'][v.level] || 'pass';
      const label = ['OK','!','NG'][v.level] || 'OK';
      const countBadge = v.count > 1 ? '<span class="val-count">×' + v.count + '</span>' : '';
      html += '<div class="check-item"><div class="check-icon ' + cls + '">' + label + '</div>'
        + '<div class="check-row"><span class="check-text">' + (v.sf||'') + ' ' + (v.message||'') + '</span>' + countBadge + '</div></div>';
    }
  }
  container.innerHTML = html;
}

// --- Report ---
function renderReport(data) {
  document.getElementById('rpt-handled').textContent = data.totalHandled;
  document.getElementById('rpt-expected').textContent = data.totalExpected;
  document.getElementById('rpt-pct').textContent = data.percentage.toFixed(1) + '%';
  document.getElementById('rpt-standards').textContent = data.standards.length;

  const bars = document.getElementById('coverage-bars');
  bars.innerHTML = '';
  for (const sc of data.standards) {
    const color = sc.percentage >= 90 ? 'var(--green)' : sc.percentage >= 70 ? 'var(--yellow)' : 'var(--red)';
    bars.innerHTML += '<div class="coverage-bar-container">'
      + '<div class="coverage-label"><span>' + sc.standard + '</span><span style="color:' + color + '">' + sc.percentage.toFixed(0) + '%</span></div>'
      + '<div class="coverage-bar"><div class="coverage-fill" style="width:' + sc.percentage + '%;background:' + color + '"></div></div></div>';
  }

  const tbody = document.getElementById('sf-tbody');
  tbody.innerHTML = '';
  for (const sf of data.sfDetail) {
    const cls = ['full','partial','none'][sf.status] || 'none';
    const label = ['FULL','PARTIAL','NONE'][sf.status] || 'NONE';
    tbody.innerHTML += '<tr><td>S' + sf.stream + 'F' + sf.function + '</td>'
      + '<td>' + sf.name + '</td><td>' + sf.direction + '</td><td>' + sf.standard + '</td>'
      + '<td><span class="impl-badge ' + cls + '">' + label + '</span></td></tr>';
  }
}

// --- Send ---
async function quickSend(name) {
  const r = await apiPost('/send', {name});
  updateSendStatus(r, name);
  setTimeout(pollTrace, 500);
}

async function rawSend() {
  const stream = parseInt(document.getElementById('send-stream').value);
  const fn = parseInt(document.getElementById('send-function').value);
  const body = document.getElementById('send-body').value;
  const r = await apiPost('/send', {stream, function: fn, body});
  updateSendStatus(r, `S${stream}F${fn}`);
  setTimeout(pollTrace, 500);
}

function updateSendStatus(r, label) {
  const el = document.getElementById('send-status');
  if (!el) return;
  const ts = new Date().toLocaleTimeString('en-US', {hour12: false});
  if (!r) {
    el.className = 'send-status error';
    el.textContent = `${ts}  Failed: ${label}`;
    return;
  }
  el.className = 'send-status success';
  const sent = r.sent || label;
  if (typeof r.replied === 'number') {
    const stream = sent.match(/^S(\d+)F/)?.[1] || '?';
    el.textContent = `${ts}  Sent ${sent} → Reply S${stream}F${r.replied}`;
  } else if (typeof r.steps === 'number') {
    el.textContent = `${ts}  Sent ${sent} (${r.steps} steps)`;
  } else {
    el.textContent = `${ts}  Sent ${sent}`;
  }
}

// --- Init ---
document.addEventListener('DOMContentLoaded', () => {
  document.querySelectorAll('.tab').forEach(tab => {
    tab.addEventListener('click', () => {
      document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
      document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
      tab.classList.add('active');
      document.getElementById(tab.dataset.tab).classList.add('active');
      if (tab.dataset.tab === 'report') {
        apiGet('/report').then(data => { if (data) renderReport(data); });
      }
    });
  });

  document.querySelectorAll('[data-quick]').forEach(btn => {
    btn.addEventListener('click', () => quickSend(btn.dataset.quick));
  });
  document.getElementById('send-btn').addEventListener('click', rawSend);

  startPolling();
});
