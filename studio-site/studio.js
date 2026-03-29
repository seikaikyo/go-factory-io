'use strict';

// Go service on Render (WebSocket + REST)
const GO_HOST = 'go-factory-io.onrender.com';
const API_BASE = 'https://' + GO_HOST;

let ws = null;
let validationLog = [];
let msgCount = 0;

// --- WebSocket ---
function connectWS() {
  const url = 'wss://' + GO_HOST + '/ws';
  setConnectionStatus(false, 'CONNECTING');

  ws = new WebSocket(url);
  ws.onopen = () => setConnectionStatus(true);
  ws.onclose = () => {
    setConnectionStatus(false);
    setTimeout(connectWS, 3000);
  };
  ws.onerror = () => setConnectionStatus(false);
  ws.onmessage = (e) => {
    try {
      const msg = JSON.parse(e.data);
      handleMessage(msg);
    } catch(err) { console.error(err); }
  };
}

function sendCmd(type, data) {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({type, data}));
  }
}

function handleMessage(msg) {
  switch(msg.type) {
    case 'trace': appendTrace(msg.data); break;
    case 'status': break;
    case 'report': renderReport(msg.data); break;
    case 'script_result': renderScriptResult(msg.data); break;
    case 'error': console.error('Studio:', msg.data.message); break;
  }
}

function setConnectionStatus(connected, text) {
  const el = document.getElementById('conn-status');
  const textEl = document.getElementById('conn-text');
  if (connected) {
    el.className = 'conn-status connected';
    textEl.textContent = 'CONNECTED';
  } else {
    el.className = 'conn-status disconnected';
    textEl.textContent = text || 'DISCONNECTED';
  }
}

// --- Trace ---
function appendTrace(entry) {
  const feed = document.getElementById('live-feed');
  if (!feed) return;

  // Clear empty state on first entry
  if (feed.querySelector('.empty-state')) {
    feed.innerHTML = '';
  }

  const ts = new Date(entry.timestamp).toLocaleTimeString('en-US', {hour12:false, fractionalSecondDigits:1});
  const sf = 'S' + entry.stream + 'F' + entry.function;
  const dirClass = entry.direction === 'tx' ? 'tx' : 'rx';
  const dirLabel = entry.direction.toUpperCase();

  let badgeClass = 'pass';
  let badgeText = 'PASS';
  if (entry.validation) {
    for (const v of entry.validation) {
      if (v.level === 2) { badgeClass = 'fail'; badgeText = 'FAIL'; break; }
      if (v.level === 1) { badgeClass = 'warn'; badgeText = 'WARN'; }
    }
    for (const v of entry.validation) {
      validationLog.push({sf, ...v});
    }
    renderValidation();
  }

  const item = document.createElement('div');
  item.className = 'feed-item';
  item.innerHTML = '<span class="feed-time">' + ts + '</span>'
    + '<span class="feed-dir ' + dirClass + '">' + dirLabel + '</span>'
    + '<span class="feed-sf">' + sf + '</span>'
    + '<span class="feed-desc">' + (entry.bodySml || '(empty)').substring(0, 60) + '</span>'
    + '<span class="feed-badge ' + badgeClass + '">' + badgeText + '</span>';
  feed.appendChild(item);
  feed.scrollTop = feed.scrollHeight;

  // Update counts
  const countEl = document.getElementById('msg-count');
  if (countEl) countEl.textContent = lastTraceId;
  const valEl = document.getElementById('val-count');
  if (valEl) valEl.textContent = validationLog.length;
}

// --- Validator ---
function renderValidation() {
  const container = document.getElementById('validation-list');
  if (!container) return;
  container.innerHTML = '';
  const recent = validationLog.slice(-50);
  for (const v of recent) {
    const levelMap = {0: 'pass', 1: 'warn', 2: 'fail'};
    const labelMap = {0: 'OK', 1: '!', 2: 'NG'};
    const cls = levelMap[v.level] || 'pass';
    const label = labelMap[v.level] || 'OK';
    const item = document.createElement('div');
    item.className = 'check-item';
    item.innerHTML = '<div class="check-icon ' + cls + '">' + label + '</div>'
      + '<div><div class="check-text">' + (v.sf || '') + ' ' + v.message + '</div></div>';
    container.appendChild(item);
  }
}

// --- Report ---
function renderReport(data) {
  document.getElementById('rpt-handled').textContent = data.totalHandled;
  document.getElementById('rpt-expected').textContent = data.totalExpected;
  document.getElementById('rpt-pct').textContent = data.percentage.toFixed(1) + '%';
  document.getElementById('rpt-standards').textContent = data.standards.length;

  const barsEl = document.getElementById('coverage-bars');
  barsEl.innerHTML = '';
  for (const sc of data.standards) {
    const color = sc.percentage >= 90 ? 'var(--green)' : sc.percentage >= 70 ? 'var(--yellow)' : 'var(--red)';
    barsEl.innerHTML += '<div class="coverage-bar-container">'
      + '<div class="coverage-label"><span>' + sc.standard + '</span><span style="color:' + color + '">' + sc.percentage.toFixed(0) + '%</span></div>'
      + '<div class="coverage-bar"><div class="coverage-fill" style="width:' + sc.percentage + '%;background:' + color + '"></div></div>'
      + '</div>';
  }

  const tbody = document.getElementById('sf-tbody');
  tbody.innerHTML = '';
  for (const sf of data.sfDetail) {
    const statusMap = {0: 'full', 1: 'partial', 2: 'none'};
    const labelMap = {0: 'FULL', 1: 'PARTIAL', 2: 'NONE'};
    const cls = statusMap[sf.status] || 'none';
    const label = labelMap[sf.status] || 'NONE';
    tbody.innerHTML += '<tr><td>S' + sf.stream + 'F' + sf.function + '</td>'
      + '<td>' + sf.name + '</td>'
      + '<td>' + sf.direction + '</td>'
      + '<td>' + sf.standard + '</td>'
      + '<td><span class="impl-badge ' + cls + '">' + label + '</span></td></tr>';
  }
}

// --- Send messages ---
function quickSend(name) {
  sendCmd('quick_send', {name});
}

function rawSend() {
  const stream = parseInt(document.getElementById('send-stream').value);
  const fn = parseInt(document.getElementById('send-function').value);
  const body = document.getElementById('send-body').value;
  sendCmd('send', {stream, function: fn, wbit: true, body});
}

// --- Init ---
document.addEventListener('DOMContentLoaded', () => {
  // Tab switching
  document.querySelectorAll('.tab').forEach(tab => {
    tab.addEventListener('click', () => {
      document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
      document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
      tab.classList.add('active');
      document.getElementById(tab.dataset.tab).classList.add('active');
      if (tab.dataset.tab === 'report') sendCmd('get_report');
    });
  });

  // Quick send buttons
  document.querySelectorAll('[data-quick]').forEach(btn => {
    btn.addEventListener('click', () => quickSend(btn.dataset.quick));
  });

  // Run script buttons
  document.querySelectorAll('[data-script]').forEach(btn => {
    btn.addEventListener('click', () => sendCmd('run_script', {index: parseInt(btn.dataset.script)}));
  });

  // Raw send
  document.getElementById('send-btn').addEventListener('click', rawSend);

  // Connect WebSocket
  connectWS();
});
