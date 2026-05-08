'use strict';

async function call(spec) { return apiPost('/send', spec); }

const CHAT_RULES = [
  {name: 'help', tk: 'help', handle: async () => ({type: 'info', text: t('chat.help')})},
  {name: 'greet', tk: 'greet', handle: async () => ({type: 'info', text: t('chat.greet')})},
  {name: 'status', tk: 'status', api: () => call({name: 'are_you_there'}), ok: 'chat.status.ok', fail: 'chat.status.fail'},
  {name: 'establish', tk: 'establish', api: () => call({name: 'establish_comm'}), ok: 'chat.establish.ok', fail: 'chat.establish.fail'},
  {name: 'online', tk: 'online', api: () => call({name: 'request_online'}), ok: 'chat.online.ok', fail: 'chat.online.fail'},
  {name: 'offline', tk: 'offline', api: () => call({name: 'request_offline'}), ok: 'chat.offline.ok', fail: 'chat.offline.fail'},
  {name: 'equipment-id', tk: 'equipment-id', api: () => call({stream: 1, function: 11, body: ''}), ok: 'chat.eqId.ok', fail: 'chat.eqId.fail'},
  {name: 'equipment-info', tk: 'equipment-info', api: () => call({stream: 1, function: 11, body: ''}), ok: 'chat.eqInfo.ok', fail: 'chat.eqInfo.fail'},
  {name: 'svreq', tk: 'svreq', api: () => call({stream: 1, function: 3, body: ''}), ok: 'chat.svreq.ok', fail: 'chat.svreq.fail'},
  {name: 'collection-event', tk: 'collection-event', api: () => call({stream: 2, function: 33, body: ''}), ok: 'chat.ceid.ok', fail: 'chat.ceid.fail'},
  {name: 'clock-get', tk: 'clock-get', api: () => call({stream: 2, function: 17, body: ''}), ok: 'chat.clockGet.ok', fail: 'chat.clockGet.fail'},
  {name: 'clock-set', tk: 'clock-set', api: () => call({stream: 2, function: 31, body: ''}), ok: 'chat.clockSet.ok', fail: 'chat.clockSet.fail'},
  {name: 'alarms', tk: 'alarms', api: () => call({name: 'alarm_report'}), ok: 'chat.alarms.ok', fail: 'chat.alarms.fail'},
  {name: 'enable-alarm', tk: 'enable-alarm', api: () => call({stream: 5, function: 3, body: ''}), ok: 'chat.enableAlarm.ok', fail: 'chat.enableAlarm.fail'},
  {name: 'disable-alarm', tk: 'disable-alarm', api: () => call({stream: 5, function: 3, body: ''}), ok: 'chat.disableAlarm.ok', fail: 'chat.disableAlarm.fail'},
  {name: 'ack-alarm', tk: 'ack-alarm', api: () => call({stream: 5, function: 5, body: ''}), ok: 'chat.ackAlarm.ok', fail: 'chat.ackAlarm.fail'},
  {name: 'events', tk: 'events', api: () => call({name: 'event_report'}), ok: 'chat.events.ok', fail: 'chat.events.fail'},
  {name: 'event-define', tk: 'event-define', api: () => call({stream: 2, function: 37, body: ''}), ok: 'chat.eventDef.ok', fail: 'chat.eventDef.fail'},
  {name: 'data-trace', tk: 'data-trace', api: () => call({stream: 6, function: 1, body: ''}), ok: 'chat.dataTrace.ok', fail: 'chat.dataTrace.fail'},
  {name: 'start', tk: 'start', api: () => call({name: 'rcmd_start'}), ok: 'chat.start.ok', fail: 'chat.start.fail'},
  {name: 'stop', tk: 'stop', api: () => call({stream: 2, function: 41, body: 'L:2 { A "STOP" L:0 }'}), ok: 'chat.stop.ok', fail: 'chat.stop.fail'},
  {name: 'pause', tk: 'pause', api: () => call({stream: 2, function: 41, body: 'L:2 { A "PAUSE" L:0 }'}), ok: 'chat.pause.ok', fail: 'chat.pause.fail'},
  {name: 'resume', tk: 'resume', api: () => call({stream: 2, function: 41, body: 'L:2 { A "RESUME" L:0 }'}), ok: 'chat.resume.ok', fail: 'chat.resume.fail'},
  {name: 'abort', tk: 'abort', api: () => call({stream: 2, function: 41, body: 'L:2 { A "ABORT" L:0 }'}), ok: 'chat.abort.ok', fail: 'chat.abort.fail'},
  {name: 'recipe-list', tk: 'recipe-list', api: () => call({stream: 7, function: 19, body: ''}), ok: 'chat.recipeList.ok', fail: 'chat.recipeList.fail'},
  {name: 'recipe-current', tk: 'recipe-current', api: () => call({stream: 7, function: 25, body: ''}), ok: 'chat.recipeCurrent.ok', fail: 'chat.recipeCurrent.fail'},
  {name: 'recipe-select', tk: 'recipe-select', api: () => call({stream: 7, function: 1, body: ''}), ok: 'chat.recipeSelect.ok', fail: 'chat.recipeSelect.fail'},
  {name: 'recipe-load', tk: 'recipe-load', api: () => call({stream: 7, function: 23, body: ''}), ok: 'chat.recipeLoad.ok', fail: 'chat.recipeLoad.fail'},
  {name: 'recipe-delete', tk: 'recipe-delete', api: () => call({stream: 7, function: 17, body: ''}), ok: 'chat.recipeDelete.ok', fail: 'chat.recipeDelete.fail'},
  {name: 'terminal-msg', tk: 'terminal-msg', api: () => call({stream: 10, function: 3, body: ''}), ok: 'chat.terminal.ok', fail: 'chat.terminal.fail'},
  {name: 'spool-toggle', tk: 'spool-toggle', api: () => call({stream: 6, function: 23, body: ''}), ok: 'chat.spool.ok', fail: 'chat.spool.fail'},
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
    if (i % 6 === 0) bubble.scrollIntoView({block: 'nearest'});
  }
  bubble.scrollIntoView({block: 'nearest'});
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

  block.scrollIntoView({block: 'start', behavior: 'smooth'});

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
        result = r ? {type: 'success', text: t(rule.ok)} : {type: 'error', text: t(rule.fail)};
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
