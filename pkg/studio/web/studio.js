'use strict';

let ws = null;
let traceLog = [];
let validationLog = [];

// --- WebSocket ---
function connectWS() {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  ws = new WebSocket(proto + '//' + location.host + '/ws');
  ws.onopen = () => {
    document.getElementById('conn-status').className = 'conn-status connected';
    document.getElementById('conn-text').textContent = 'CONNECTED';
  };
  ws.onclose = () => {
    document.getElementById('conn-status').className = 'conn-status disconnected';
    document.getElementById('conn-text').textContent = 'DISCONNECTED';
    setTimeout(connectWS, 2000);
  };
  ws.onmessage = (e) => {
    try { handleMessage(JSON.parse(e.data)); } catch(err) { console.error(err); }
  };
}

function sendCmd(type, data) {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({type, data}));
  }
}

// --- Message Router ---
function handleMessage(msg) {
  switch(msg.type) {
    case 'trace': appendTrace(msg.data); break;
    case 'status': updateDashboard(msg.data); break;
    case 'report': renderReport(msg.data); break;
    case 'script_result': renderScriptResult(msg.data); break;
    case 'error': showError(msg.data.message); break;
  }
}

// --- Dashboard ---
function updateDashboard(data) {
  const el = document.getElementById('trace-count');
  if (el) el.textContent = data.traceCount || 0;
}

// --- Trace ---
function appendTrace(entry) {
  traceLog.push(entry);
  const feed = document.getElementById('live-feed');
  if (!feed) return;

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
  }

  // Also add to validation tab
  if (entry.validation) {
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

  // Update count
  const countEl = document.getElementById('msg-count');
  if (countEl) countEl.textContent = traceLog.length;
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
  // Stats
  document.getElementById('rpt-handled').textContent = data.totalHandled;
  document.getElementById('rpt-expected').textContent = data.totalExpected;
  document.getElementById('rpt-pct').textContent = data.percentage.toFixed(1) + '%';
  document.getElementById('rpt-standards').textContent = data.standards.length;

  // Coverage bars
  const barsEl = document.getElementById('coverage-bars');
  barsEl.innerHTML = '';
  for (const sc of data.standards) {
    const color = sc.percentage >= 90 ? 'var(--green)' : sc.percentage >= 70 ? 'var(--yellow)' : 'var(--red)';
    barsEl.innerHTML += '<div class="coverage-bar-container">'
      + '<div class="coverage-label"><span>' + sc.standard + '</span><span style="color:' + color + '">' + sc.percentage.toFixed(0) + '%</span></div>'
      + '<div class="coverage-bar"><div class="coverage-fill" style="width:' + sc.percentage + '%;background:' + color + '"></div></div>'
      + '</div>';
  }

  // S/F table
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

function renderScriptResult(result) {
  let html = '<div class="card"><div class="card-header">Script: ' + result.name
    + ' <span style="color:' + (result.failed > 0 ? 'var(--red)' : 'var(--green)') + '">'
    + result.passed + ' passed, ' + result.failed + ' failed</span></div><div class="card-body">';
  for (const s of result.steps) {
    const cls = s.status === 'pass' ? 'pass' : s.status === 'fail' ? 'fail' : 'warn';
    html += '<div class="check-item"><div class="check-icon ' + cls + '">'
      + (s.status === 'pass' ? 'OK' : s.status === 'fail' ? 'NG' : '!') + '</div>'
      + '<div><div class="check-text">Step ' + s.step + ': ' + s.action + '</div>'
      + '<div class="check-detail">' + s.detail + '</div></div></div>';
  }
  html += '</div></div>';
  document.getElementById('script-results').innerHTML = html;
}

function showError(msg) {
  console.error('Studio error:', msg);
}

// --- Tab switching ---
document.addEventListener('DOMContentLoaded', () => {
  document.querySelectorAll('.tab').forEach(tab => {
    tab.addEventListener('click', () => {
      document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
      document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
      tab.classList.add('active');
      document.getElementById(tab.dataset.tab).classList.add('active');

      // Load report on first visit
      if (tab.dataset.tab === 'report' && !document.getElementById('rpt-handled').textContent) {
        sendCmd('get_report');
      }
    });
  });

  // Quick send buttons
  document.querySelectorAll('[data-quick]').forEach(btn => {
    btn.addEventListener('click', () => sendCmd('quick_send', {name: btn.dataset.quick}));
  });

  // Fault buttons
  document.querySelectorAll('[data-fault]').forEach(btn => {
    btn.addEventListener('click', () => sendCmd('fault', {type: btn.dataset.fault}));
  });

  // Send raw message
  document.getElementById('send-btn').addEventListener('click', () => {
    const stream = parseInt(document.getElementById('send-stream').value);
    const fn = parseInt(document.getElementById('send-function').value);
    const body = document.getElementById('send-body').value;
    sendCmd('send', {stream, function: fn, wbit: true, body});
  });

  // Run script
  document.querySelectorAll('[data-script]').forEach(btn => {
    btn.addEventListener('click', () => sendCmd('run_script', {index: parseInt(btn.dataset.script)}));
  });

  connectWS();
});
