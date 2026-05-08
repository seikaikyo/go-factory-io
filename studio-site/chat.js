'use strict';

async function call(spec) {
  const r = await apiPost('/send', spec);
  // Sync the Dashboard immediately so state pills / wafer journey /
  // jobs tree reflect the same mutation the bot reply describes,
  // instead of waiting up to 3s for the next /state poll.
  if (r && typeof renderDashboardState === 'function') {
    renderDashboardState(r);
  }
  return r;
}

const CHAT_RULES = [
  {name: 'help', tk: 'help', handle: async () => ({type: 'info', text: t('chat.help')})},
  {name: 'greet', tk: 'greet', handle: async () => ({type: 'info', text: t('chat.greet')})},
  {name: 'status', tk: 'status', api: () => call({name: 'are_you_there'}), ok: 'chat.status.ok', fail: 'chat.status.fail', formatOk: r => t('chat.status.ok', r.controlState, r.processState, r.online)},
  {name: 'establish', tk: 'establish', api: () => call({name: 'establish_comm'}), ok: 'chat.establish.ok', fail: 'chat.establish.fail'},
  {name: 'online', tk: 'online', api: () => call({name: 'request_online'}), ok: 'chat.online.ok', fail: 'chat.online.fail', formatOk: r => t('chat.online.ok', r.controlState)},
  {name: 'offline', tk: 'offline', api: () => call({name: 'request_offline'}), ok: 'chat.offline.ok', fail: 'chat.offline.fail'},
  {name: 'equipment-id', tk: 'equipment-id', api: () => call({stream: 1, function: 11, body: ''}), ok: 'chat.eqId.ok', fail: 'chat.eqId.fail', formatOk: r => t('chat.eqId.ok', r.mdln, r.softrev)},
  {name: 'equipment-info', tk: 'equipment-info', api: () => call({stream: 1, function: 11, body: ''}), ok: 'chat.eqInfo.ok', fail: 'chat.eqInfo.fail'},
  {name: 'svreq', tk: 'svreq', api: () => call({stream: 1, function: 3, body: ''}), ok: 'chat.svreq.ok', fail: 'chat.svreq.fail'},
  {name: 'collection-event', tk: 'collection-event', api: () => call({stream: 2, function: 33, body: ''}), ok: 'chat.ceid.ok', fail: 'chat.ceid.fail'},
  {name: 'clock-get', tk: 'clock-get', api: () => call({stream: 2, function: 17, body: ''}), ok: 'chat.clockGet.ok', fail: 'chat.clockGet.fail', formatOk: r => t('chat.clockGet.ok', r.clockDriftMs)},
  {name: 'clock-set', tk: 'clock-set', api: () => call({stream: 2, function: 31, body: ''}), ok: 'chat.clockSet.ok', fail: 'chat.clockSet.fail'},
  // Specific alarm subcommands first; generic 'alarms' must come last so
  // chip text like "Ack Alarm" doesn't fall through to the listing intent.
  {name: 'ack-alarm', tk: 'ack-alarm', api: () => call({stream: 5, function: 5, body: ''}), ok: 'chat.ackAlarm.ok', fail: 'chat.ackAlarm.fail', formatOk: r => t('chat.ackAlarm.ok', r.acked && r.acked.name, r.unackedCount)},
  {name: 'enable-alarm', tk: 'enable-alarm', api: () => call({stream: 5, function: 3, body: ''}), ok: 'chat.enableAlarm.ok', fail: 'chat.enableAlarm.fail'},
  {name: 'disable-alarm', tk: 'disable-alarm', api: () => call({stream: 5, function: 3, body: ''}), ok: 'chat.disableAlarm.ok', fail: 'chat.disableAlarm.fail'},
  {name: 'alarms', tk: 'alarms', api: () => call({name: 'alarm_report'}), ok: 'chat.alarms.ok', fail: 'chat.alarms.fail', formatOk: r => t('chat.alarms.ok', r.unackedCount, r.alarmCount)},
  {name: 'events', tk: 'events', api: () => call({name: 'event_report'}), ok: 'chat.events.ok', fail: 'chat.events.fail', formatOk: r => t('chat.events.ok', r.eventCount, (r.recentEvents || []).slice(0, 3))},
  {name: 'event-define', tk: 'event-define', api: () => call({stream: 2, function: 37, body: ''}), ok: 'chat.eventDef.ok', fail: 'chat.eventDef.fail'},
  {name: 'data-trace', tk: 'data-trace', api: () => call({stream: 6, function: 1, body: ''}), ok: 'chat.dataTrace.ok', fail: 'chat.dataTrace.fail'},
  {name: 'start', tk: 'start', api: () => call({name: 'rcmd_start'}), ok: 'chat.start.ok', fail: 'chat.start.fail', formatOk: r => t('chat.start.ok', r.processState)},
  {name: 'stop', tk: 'stop', api: () => call({name: 'rcmd_stop'}), ok: 'chat.stop.ok', fail: 'chat.stop.fail', formatOk: r => t('chat.stop.ok', r.processState)},
  {name: 'pause', tk: 'pause', api: () => call({name: 'rcmd_pause'}), ok: 'chat.pause.ok', fail: 'chat.pause.fail', formatOk: r => t('chat.pause.ok', r.processState)},
  {name: 'resume', tk: 'resume', api: () => call({name: 'rcmd_resume'}), ok: 'chat.resume.ok', fail: 'chat.resume.fail', formatOk: r => t('chat.resume.ok', r.processState)},
  {name: 'abort', tk: 'abort', api: () => call({name: 'rcmd_abort'}), ok: 'chat.abort.ok', fail: 'chat.abort.fail', formatOk: r => t('chat.abort.ok', r.processState, r.unackedCount)},
  {name: 'recipe-list', tk: 'recipe-list', api: () => call({stream: 7, function: 19, body: ''}), ok: 'chat.recipeList.ok', fail: 'chat.recipeList.fail', formatOk: r => t('chat.recipeList.ok', (r.recipes || []).join(', '), r.currentRecipe)},
  {name: 'recipe-current', tk: 'recipe-current', api: () => call({stream: 7, function: 25, body: ''}), ok: 'chat.recipeCurrent.ok', fail: 'chat.recipeCurrent.fail', formatOk: r => t('chat.recipeCurrent.ok', r.currentRecipe)},
  {name: 'recipe-select', tk: 'recipe-select', api: () => call({stream: 7, function: 1, body: ''}), ok: 'chat.recipeSelect.ok', fail: 'chat.recipeSelect.fail', formatOk: r => t('chat.recipeSelect.ok', r.prevRecipe, r.currentRecipe)},
  {name: 'recipe-load', tk: 'recipe-load', api: () => call({stream: 7, function: 23, body: ''}), ok: 'chat.recipeLoad.ok', fail: 'chat.recipeLoad.fail'},
  {name: 'recipe-delete', tk: 'recipe-delete', api: () => call({stream: 7, function: 17, body: ''}), ok: 'chat.recipeDelete.ok', fail: 'chat.recipeDelete.fail'},
  {name: 'terminal-msg', tk: 'terminal-msg', api: () => call({stream: 10, function: 3, body: ''}), ok: 'chat.terminal.ok', fail: 'chat.terminal.fail'},
  {name: 'spool-toggle', tk: 'spool-toggle', api: () => call({stream: 6, function: 23, body: ''}), ok: 'chat.spool.ok', fail: 'chat.spool.fail'},
  // Standard-specific intents (E5 / E37 / E40 / E84 / E87 / E90 / E94 / E116)
  {name: 'linktest', tk: 'linktest', api: () => call({name: 'linktest'}), ok: 'chat.linktest.ok', fail: 'chat.linktest.fail', formatOk: r => t('chat.linktest.ok', r.latencyMs)},
  {name: 'format-verify', tk: 'format-verify', api: () => call({stream: 1, function: 65, body: ''}), ok: 'chat.formatVerify.ok', fail: 'chat.formatVerify.fail', formatOk: r => t('chat.formatVerify.ok', r.verifyCount)},
  {name: 'material-handoff', tk: 'material-handoff', api: () => call({stream: 2, function: 19, body: ''}), ok: 'chat.materialHandoff.ok', fail: 'chat.materialHandoff.fail'},
  {name: 'carrier-bind', tk: 'carrier-bind', api: () => call({stream: 3, function: 1, body: ''}), ok: 'chat.carrierBind.ok', fail: 'chat.carrierBind.fail', formatOk: r => t('chat.carrierBind.ok', r.carrierId, r.portId)},
  {name: 'substrate-state', tk: 'substrate-state', api: () => call({stream: 3, function: 31, body: ''}), ok: 'chat.substrateState.ok', fail: 'chat.substrateState.fail', formatOk: r => t('chat.substrateState.ok', r.substrateState, (r.substrateHistory || [])[0])},
  {name: 'control-job', tk: 'control-job', api: () => call({stream: 16, function: 27, body: ''}), ok: 'chat.controlJob.ok', fail: 'chat.controlJob.fail', formatOk: r => t('chat.controlJob.ok', r.newCjid, (r.adoptedPjids || []).length, r.cjobCount)},
  {name: 'ept-report', tk: 'ept-report', api: () => call({stream: 6, function: 19, body: ''}), ok: 'chat.eptReport.ok', fail: 'chat.eptReport.fail', formatOk: r => t('chat.eptReport.ok', r.oee)},
  {name: 'process-job', tk: 'process-job', api: () => call({stream: 16, function: 11, body: ''}), ok: 'chat.processJob.ok', fail: 'chat.processJob.fail', formatOk: r => t('chat.processJob.ok', r.newPjid, r.currentRecipe, r.parentCjid)},
];

function matchChatRule(query) {
  const q = query.toLowerCase().trim();
  const sf = q.match(/^[Ss](\d+)[Ff](\d+)/);
  if (sf) return {kind: 'sf', stream: parseInt(sf[1]), fn: parseInt(sf[2])};
  for (const rule of CHAT_RULES) {
    const kws = (I18N_KEYWORDS && I18N_KEYWORDS[rule.tk]) || [];
    for (const kw of kws) {
      if (q.includes(kw.toLowerCase())) return {kind: 'rule', rule};
    }
  }
  return null;
}

function chatBubble(role, text) {
  const wrap = document.createElement('div');
  wrap.className = 'chat-msg ' + role;
  const avatar = document.createElement('div');
  avatar.className = 'chat-avatar ' + role;
  avatar.textContent = role === 'user' ? 'U' : 'AI';
  const bubble = document.createElement('div');
  bubble.className = 'chat-bubble';
  bubble.textContent = text;
  wrap.appendChild(avatar);
  wrap.appendChild(bubble);
  return {wrap, bubble};
}

function startThinking(bubble) {
  bubble.classList.add('thinking');
  bubble.textContent = t('chat.thinking');
  let dots = 0;
  const id = setInterval(() => {
    dots = (dots + 1) % 4;
    bubble.textContent = t('chat.thinking') + '.'.repeat(dots);
  }, 300);
  return () => { clearInterval(id); bubble.classList.remove('thinking'); bubble.textContent = ''; };
}

async function streamText(bubble, text, speed = 12) {
  bubble.textContent = '';
  bubble.classList.add('streaming');
  for (let i = 0; i < text.length; i++) {
    bubble.textContent += text[i];
    const ch = text[i];
    if (ch !== ' ' && ch !== '\n') {
      const delay = ch === '。' || ch === '.' ? speed * 4 : speed;
      await new Promise(r => setTimeout(r, delay));
    }
  }
  bubble.classList.remove('streaming');
}

async function chatSend(prefilled) {
  const input = document.getElementById('chat-input');
  const text = (prefilled || input.value).trim();
  if (!text) return;
  if (!prefilled) input.value = '';
  input.disabled = true;
  document.getElementById('chat-send-btn').disabled = true;

  const feed = document.getElementById('chat-feed');
  const empty = feed.querySelector('.empty-state');
  if (empty) empty.remove();

  // Newest at top: prepend a paired block (user + bot)
  const block = document.createElement('div');
  block.className = 'chat-block';
  feed.prepend(block);

  const u = chatBubble('user', text);
  block.appendChild(u.wrap);
  const b = chatBubble('bot', '');
  block.appendChild(b.wrap);

  const stopThink = startThinking(b.bubble);
  await new Promise(r => setTimeout(r, 350 + Math.random() * 400));

  let result;
  const matched = matchChatRule(text);
  try {
    if (!matched) {
      result = {type: 'info', text: t('chat.fallback')};
    } else if (matched.kind === 'sf') {
      const r = await call({stream: matched.stream, function: matched.fn, body: ''});
      if (!r) result = {type: 'error', text: t('chat.sfDirect.fail', matched.stream, matched.fn)};
      else {
        const reply = r.replied != null ? r.replied : (matched.fn + 1);
        result = {type: 'success', text: t('chat.sfDirect.ok', matched.stream, matched.fn, reply)};
      }
    } else {
      const rule = matched.rule;
      if (rule.handle) {
        result = await rule.handle();
      } else {
        const r = await rule.api();
        if (r) {
          const text = rule.formatOk ? rule.formatOk(r) : t(rule.ok);
          result = {type: 'success', text};
        } else {
          result = {type: 'error', text: t(rule.fail)};
        }
      }
    }
  } catch (e) {
    result = {type: 'error', text: t('chat.execError') + (e.message || e)};
  }

  stopThink();
  b.bubble.classList.add('chat-' + result.type);
  await streamText(b.bubble, result.text);

  input.disabled = false;
  document.getElementById('chat-send-btn').disabled = false;
}

document.addEventListener('DOMContentLoaded', () => {
  const sendBtn = document.getElementById('chat-send-btn');
  const input = document.getElementById('chat-input');
  if (!sendBtn || !input) return;
  sendBtn.addEventListener('click', () => chatSend());
  input.addEventListener('keypress', e => {
    if (e.key === 'Enter') { e.preventDefault(); chatSend(); }
  });
  document.querySelectorAll('[data-chat-suggest]').forEach(btn => {
    btn.addEventListener('click', () => chatSend(btn.textContent.trim()));
  });
});
