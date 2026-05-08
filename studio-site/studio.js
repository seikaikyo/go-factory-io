'use strict';

// Backend: dashai-api Python simulator
const API_BASE = 'https://dashai-api.onrender.com/factory/api/v1/equipment/studio';

let lastTraceId = 0;
let validationLog = [];
let currentSimFilter = 'all';

function updateSimCounts() {
  const sim = document.getElementById('sim-feed');
  if (!sim) return;
  const tx = sim.querySelectorAll('.sim-msg.tx').length;
  const rx = sim.querySelectorAll('.sim-msg.rx').length;
  const txEl = document.getElementById('sim-tx-count');
  const rxEl = document.getElementById('sim-rx-count');
  if (txEl) txEl.textContent = tx + ' TX';
  if (rxEl) rxEl.textContent = rx + ' RX';
}

function applySimFilter(filter) {
  currentSimFilter = filter;
  document.querySelectorAll('.sim-filter-btn').forEach(b => {
    b.classList.toggle('active', b.dataset.filter === filter);
  });
  document.querySelectorAll('#sim-feed .sim-msg').forEach(m => {
    const show = filter === 'all' || m.dataset.dir === filter;
    m.style.display = show ? '' : 'none';
  });
}

function clearSimFeed() {
  const sim = document.getElementById('sim-feed');
  if (!sim) return;
  sim.innerHTML = '<div class="empty-state" style="padding:20px;font-size:12px">' +
    ((typeof t === 'function') ? t('sim.feedEmpty') : 'No messages yet.') + '</div>';
  updateSimCounts();
}

const SCENARIOS = {
  happy: [
    {chip: 'establish', key: 'dash.runDemo.step.handshake'},       // S1F13
    {chip: 'online', key: 'dash.runDemo.step.online'},             // S1F17
    {chip: 'clockSet', key: 'dash.runDemo.step.clock'},            // S2F31
    {chip: 'recipeSelect', key: 'dash.runDemo.step.recipe'},       // S7F1
    {chip: 'carrierBind', key: 'dash.runDemo.step.bind'},          // S3F1 → AtSource
    {chip: 'materialHandoff', key: 'dash.runDemo.step.handoff'},   // S2F19 → Loading
    {chip: 'processJob', key: 'dash.runDemo.step.pjob'},           // S16F11
    {chip: 'controlJob', key: 'dash.runDemo.step.cjob'},           // S16F27 adopts PJ
    {chip: 'start', key: 'dash.runDemo.step.start'},               // S2F41 START → InProcess
    {chip: 'eptReport', key: 'dash.runDemo.step.ept'},             // S6F19 OEE
    {chip: 'stop', key: 'dash.runDemo.step.stop'},                 // S2F41 STOP → Processed
    {chip: 'substrateState', key: 'dash.runDemo.step.unload'},     // S3F31 → AtDestination
    {chip: 'ackAlarm', key: 'dash.runDemo.step.ack'},              // S5F5
    {chip: 'status', key: 'dash.runDemo.step.status'},             // S1F1
  ],
  pause: [
    {chip: 'carrierBind', key: 'dash.runDemo.step.bind'},
    {chip: 'materialHandoff', key: 'dash.runDemo.step.handoff'},
    {chip: 'processJob', key: 'dash.runDemo.step.pjob'},
    {chip: 'controlJob', key: 'dash.runDemo.step.cjob'},
    {chip: 'start', key: 'dash.runDemo.step.start'},
    {chip: 'pause', key: 'dash.runDemo.step.pause'},               // operator break
    {chip: 'status', key: 'dash.runDemo.step.statusMid'},          // confirm paused
    {chip: 'resume', key: 'dash.runDemo.step.resume'},             // back to running
    {chip: 'stop', key: 'dash.runDemo.step.stop'},
    {chip: 'substrateState', key: 'dash.runDemo.step.unload'},
    {chip: 'status', key: 'dash.runDemo.step.status'},
  ],
  abort: [
    {chip: 'carrierBind', key: 'dash.runDemo.step.bind'},
    {chip: 'materialHandoff', key: 'dash.runDemo.step.handoff'},
    {chip: 'processJob', key: 'dash.runDemo.step.pjob'},
    {chip: 'controlJob', key: 'dash.runDemo.step.cjob'},
    {chip: 'start', key: 'dash.runDemo.step.start'},
    {chip: 'abort', key: 'dash.runDemo.step.abort'},               // emergency stop
    {chip: 'alarms', key: 'dash.runDemo.step.alarmCheck'},         // query what fired
    {chip: 'ackAlarm', key: 'dash.runDemo.step.ack'},
    {chip: 'status', key: 'dash.runDemo.step.status'},
  ],
};

async function runScenario(name) {
  const buttons = document.querySelectorAll('.scenario-btn');
  if ([...buttons].some(b => b.disabled)) return;
  const steps = SCENARIOS[name];
  if (!steps) return;
  const tt = (typeof t === 'function') ? t : (k => k);
  const status = document.getElementById('scenario-status');
  buttons.forEach(b => { b.disabled = true; b.classList.toggle('running', b.dataset.scenario === name); });

  // Fresh slate: reset accumulated state from previous runs so the
  // story (PJ created → CJ adopts it → wafer journey through all 5
  // stops) reads cleanly every time.
  if (status) status.textContent = tt('dash.runDemo.resetting');
  try {
    const r = await apiPost('/reset', {});
    if (r && typeof renderDashboardState === 'function') renderDashboardState(r);
    // Clear chat feed too so the new scenario isn't mixed with old replies
    const feed = document.getElementById('chat-feed');
    if (feed) feed.innerHTML = '<div class="empty-state" style="padding:40px;text-align:center;color:var(--text-dim);font-size:12px">' + tt('chat.empty') + '</div>';
    if (typeof pollTrace === 'function') await pollTrace();
  } catch (e) {}
  await new Promise(r => setTimeout(r, 600));

  for (let i = 0; i < steps.length; i++) {
    if (status) status.textContent = tt('dash.runDemo.running', i + 1, steps.length, tt(steps[i].key));
    const chipBtn = document.querySelector(`[data-i18n='chat.suggest.${steps[i].chip}']`);
    if (chipBtn) chipBtn.click();
    // call() inside chat.js syncs renderDashboardState + pollTrace
    // from the response, so we just need to give the chat reply
    // time to stream out before firing the next chip.
    await new Promise(r => setTimeout(r, 2800));
  }
  if (status) status.textContent = tt('dash.runDemo.done');
  setTimeout(() => {
    if (status) status.textContent = '';
    buttons.forEach(b => { b.disabled = false; b.classList.remove('running'); });
  }, 2500);
}

function loadTemplate(btn) {
  document.getElementById('send-stream').value = btn.dataset.stream;
  document.getElementById('send-function').value = btn.dataset.function;
  document.getElementById('send-body').value = btn.dataset.body || '';
  document.querySelectorAll('.quick-btn').forEach(b => b.classList.remove('active'));
  btn.classList.add('active');
  const status = document.getElementById('send-status');
  if (status) {
    const tpl = btn.dataset.template;
    const tt = (typeof t === 'function') ? t : (k => k);
    status.className = 'send-status info';
    status.textContent = tt('sim.templateLoaded', tpl);
  }
}

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

async function pollState() {
  const state = await apiGet('/state');
  if (!state) return;
  renderDashboardState(state);
}

function startPolling() {
  // Single initial fetch only — no setInterval. State updates flow
  // from chat / raw send response data via renderDashboardState(),
  // and trace updates flow from explicit pollTrace() calls after
  // each user action. Avoids constant polling that Vercel BotID
  // flags as DDoS-style traffic.
  pollTrace();
  pollState();
}

function renderDashboardState(s) {
  const tt = (typeof t === 'function') ? t : (k => k);

  // Process state pill
  const proc = document.getElementById('state-process');
  const procVal = document.getElementById('state-process-val');
  if (proc && procVal) {
    const ps = (s.processState || 'IDLE').toLowerCase();
    proc.className = 'state-pill state-process state-process-' + ps;
    procVal.textContent = tt('dash.processState.' + ps) || s.processState;
  }

  // Online pill
  const online = document.getElementById('state-online');
  const onlineVal = document.getElementById('state-online-val');
  if (online && onlineVal) {
    online.className = 'state-pill ' + (s.online ? 'state-online' : 'state-offline');
    onlineVal.textContent = s.online
      ? tt('dash.online') + ' / ' + (s.controlState || 'REMOTE')
      : tt('dash.offline');
  }

  // Recipe pill
  const recipeVal = document.getElementById('state-recipe-val');
  if (recipeVal) recipeVal.textContent = s.currentRecipe || '—';

  // Alarm pill
  const alarm = document.getElementById('state-alarm');
  const alarmVal = document.getElementById('state-alarm-val');
  if (alarm && alarmVal) {
    const unacked = s.unackedCount || 0;
    alarm.className = 'state-pill ' + (unacked > 0 ? 'state-alarm-active' : 'state-alarm-clear');
    alarmVal.textContent = unacked > 0
      ? tt('dash.alarms.unacked', unacked)
      : tt('dash.alarms.clear');
  }

  // Substrate journey
  document.querySelectorAll('.journey-stop').forEach(stop => {
    stop.classList.remove('active', 'reached');
  });
  const order = ['AtSource', 'Loading', 'InProcess', 'Processed', 'AtDestination'];
  const idx = order.indexOf(s.substrateState);
  if (idx >= 0) {
    document.querySelectorAll('.journey-stop').forEach((stop, i) => {
      if (i < idx) stop.classList.add('reached');
      else if (i === idx) stop.classList.add('active');
    });
  }

  // Active jobs tree
  const tree = document.getElementById('jobs-tree');
  const jobsCount = document.getElementById('jobs-count');
  if (tree && jobsCount) {
    const cjobs = s.cjobs || [];
    const totalCj = s.cjobCount || 0;
    const totalPj = s.pjobCount || 0;
    jobsCount.textContent = totalCj + ' CJ / ' + totalPj + ' PJ';
    if (cjobs.length === 0 && totalPj === 0) {
      tree.innerHTML = '<div class="empty-state">' + tt('dash.noActiveJobs') + '</div>';
    } else {
      let html = '';
      for (const cj of cjobs) {
        html += '<div class="job-cj"><span class="job-icon">CJ</span><span class="job-id">' + cj.id + '</span>';
        if (cj.pjobIds && cj.pjobIds.length) {
          html += '<div class="job-children">';
          for (const pjid of cj.pjobIds) {
            html += '<div class="job-pj"><span class="job-icon pj">PJ</span><span class="job-id">' + pjid + '</span></div>';
          }
          html += '</div>';
        }
        html += '</div>';
      }
      // Orphan PJs (created without a CJ to adopt them yet)
      if (totalPj > cjobs.reduce((a, c) => a + (c.pjobIds || []).length, 0)) {
        html += '<div class="job-orphan-hint">' + tt('dash.orphanPjHint') + '</div>';
      }
      tree.innerHTML = html;
    }
  }

  // Recent activity timeline
  const list = document.getElementById('activity-list');
  if (list) {
    const events = s.recentEvents || [];
    if (events.length === 0) {
      list.innerHTML = '<div class="empty-state">' + tt('dash.noActivity') + '</div>';
    } else {
      list.innerHTML = events.map(e => {
        const ts = new Date(e.time).toLocaleTimeString('en-US', {hour12: false});
        const detail = e.detail ? '<span class="ev-detail">' + e.detail + '</span>' : '';
        return '<div class="ev-row"><span class="ev-time">' + ts + '</span>'
          + '<span class="ev-type ev-' + e.type.toLowerCase() + '">' + e.type + '</span>'
          + detail + '</div>';
      }).join('');
    }
  }

  // Event count stat card
  const ec = document.getElementById('event-count');
  if (ec) ec.textContent = s.eventCount || 0;
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
    bubble.dataset.dir = dirClass;
    const rawBody = entry.bodySml || '';
    const isEmpty = !rawBody || rawBody === '(empty)' || rawBody === '(no payload)';
    const bodyHtml = isEmpty
      ? '<div class="sim-msg-body sim-msg-body-empty">' + ((typeof t === 'function') ? t('sim.noPayload') : 'no payload') + '</div>'
      : '<div class="sim-msg-body">' + rawBody + '</div>';
    bubble.innerHTML = '<div class="sim-msg-meta"><span>' + entry.direction.toUpperCase() + ' ' + sf + '</span><span class="sim-msg-time">' + ts + '</span></div>' + bodyHtml;
    sim.prepend(bubble);
    if (currentSimFilter !== 'all' && currentSimFilter !== dirClass) {
      bubble.style.display = 'none';
    }
    updateSimCounts();
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

  document.querySelectorAll('.quick-btn[data-template]').forEach(btn => {
    btn.addEventListener('click', () => loadTemplate(btn));
  });
  document.getElementById('send-btn').addEventListener('click', rawSend);
  document.querySelectorAll('.sim-filter-btn').forEach(btn => {
    btn.addEventListener('click', () => applySimFilter(btn.dataset.filter));
  });
  const clearBtn = document.getElementById('sim-feed-clear');
  if (clearBtn) clearBtn.addEventListener('click', clearSimFeed);
  document.querySelectorAll('.scenario-btn').forEach(btn => {
    btn.addEventListener('click', () => runScenario(btn.dataset.scenario));
  });

  startPolling();
});
