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
  text.textContent = connected ? 'CONNECTED' : 'DISCONNECTED';
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

  const c = document.getElementById('msg-count');
  if (c) c.textContent = lastTraceId;
  const v = document.getElementById('val-count');
  if (v) v.textContent = validationLog.length;
}

// --- Validator ---
function renderValidation() {
  const container = document.getElementById('validation-list');
  if (!container) return;
  container.innerHTML = '';
  for (const v of validationLog.slice(-50)) {
    const cls = ['pass','warn','fail'][v.level] || 'pass';
    const label = ['OK','!','NG'][v.level] || 'OK';
    const el = document.createElement('div');
    el.className = 'check-item';
    el.innerHTML = '<div class="check-icon ' + cls + '">' + label + '</div>'
      + '<div><div class="check-text">' + (v.sf||'') + ' ' + v.message + '</div></div>';
    container.appendChild(el);
  }
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
  await apiPost('/send', {name});
  setTimeout(pollTrace, 500);
}

async function rawSend() {
  const stream = parseInt(document.getElementById('send-stream').value);
  const fn = parseInt(document.getElementById('send-function').value);
  const body = document.getElementById('send-body').value;
  await apiPost('/send', {stream, function: fn, body});
  setTimeout(pollTrace, 500);
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
