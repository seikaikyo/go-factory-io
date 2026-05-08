'use strict';

const CHAT_INTENTS = [
  {
    name: 'help',
    keywords: ['help', '幫助', '指令', '可以做什麼', '能做什麼', '會什麼'],
    handle: async () => ({
      type: 'info',
      text: '我可以幫你處理這些事情：\n\n  • 機台狀況 / 在嗎 → equipment status\n  • 建立連線 → S1F13 establish communication\n  • 上線 / 下線 → request online/offline\n  • 警報 / 異常 → alarm report\n  • 事件回報 → event collection report\n  • 啟動 / 開始 → RCMD START\n  • 直接打 SECS 訊息 → 例如 S1F1\n\n直接用中文或英文都可以，不用擔心格式。'
    })
  },
  {
    name: 'greet',
    keywords: ['你好', '哈囉', '嗨', 'hi', 'hello'],
    handle: async () => ({type: 'info', text: '你好，我是 SECSGEM 助理。可以幫你查機台狀況、警報、控制 recipe。輸入「幫助」看完整指令清單。'})
  },
  {
    name: 'status',
    keywords: ['機台狀況', '狀況', '狀態', '現在怎樣', '在嗎', '活著', 'status', 'alive'],
    handle: async () => {
      const r = await apiPost('/send', {name: 'are_you_there'});
      if (!r) return {type: 'error', text: '無法查詢機台狀況，後端沒有回應。請檢查連線狀態。'};
      return {
        type: 'success',
        text: `機台目前在線 ✓\n\n  • 查詢路徑：S1F1 (Are You There) → S1F2 (On Line Data)\n  • 完成步驟：${r.steps || 1}\n\n設備正常回應中，可以執行下一步操作。`
      };
    }
  },
  {
    name: 'establish',
    keywords: ['建立連線', '連線', 'establish', 'connect', '建立通訊'],
    handle: async () => {
      const r = await apiPost('/send', {name: 'establish_comm'});
      if (!r) return {type: 'error', text: '建立連線失敗。'};
      return {type: 'success', text: `通訊建立成功 ✓\n\n  • S1F13 (Establish Communications Request) → S1F14 (COMMACK)\n  • 完成 ${r.steps || 0} 個交握步驟\n\nHSMS 連線已就緒，可以開始下指令。`};
    }
  },
  {
    name: 'online',
    keywords: ['上線', 'online', 'go online', 'request online'],
    handle: async () => {
      const r = await apiPost('/send', {name: 'request_online'});
      if (!r) return {type: 'error', text: '上線請求失敗。'};
      return {type: 'success', text: `上線請求已送出 ✓\n\n  • S1F17 (Request ON-LINE) → S1F18 (ONLACK)\n  • 完成 ${r.steps || 0} 個步驟\n\n機台現在處於 ON-LINE 狀態。`};
    }
  },
  {
    name: 'offline',
    keywords: ['下線', 'offline', 'request offline'],
    handle: async () => {
      const r = await apiPost('/send', {name: 'request_offline'});
      if (!r) return {type: 'error', text: '下線請求失敗。'};
      return {type: 'success', text: `下線請求已送出 ✓\n\n  • S1F15 (Request OFF-LINE) → S1F16 (OFLACK)\n  • 完成 ${r.steps || 0} 個步驟\n\n機台已切換到 OFF-LINE。`};
    }
  },
  {
    name: 'alarms',
    keywords: ['警報', 'alarm', '異常', '報警', '錯誤', '有錯嗎', '有事嗎', '問題'],
    handle: async () => {
      const r = await apiPost('/send', {name: 'alarm_report'});
      if (!r) return {type: 'error', text: '警報查詢失敗。'};
      return {type: 'success', text: `警報回報已觸發 ✓\n\n  • S5F1 (Alarm Sent) 已送出\n  • 完成 ${r.steps || 0} 個步驟\n\n切到 SIMULATOR tab 可以看到完整對話 trace。`};
    }
  },
  {
    name: 'events',
    keywords: ['事件', 'event', '回報', 'event report'],
    handle: async () => {
      const r = await apiPost('/send', {name: 'event_report'});
      if (!r) return {type: 'error', text: '事件回報失敗。'};
      return {type: 'success', text: `事件回報已觸發 ✓\n\n  • S6F11 (Event Report Send) → S6F12 (Event ACK)\n  • 完成 ${r.steps || 0} 個步驟\n\nCEID 訂閱資料已收集回 host。`};
    }
  },
  {
    name: 'start',
    keywords: ['啟動', '開始', 'start', 'run', 'rcmd start'],
    handle: async () => {
      const r = await apiPost('/send', {name: 'rcmd_start'});
      if (!r) return {type: 'error', text: '啟動指令失敗。'};
      return {type: 'success', text: `RCMD START 已送出 ✓\n\n  • S2F41 (Host Command Send) → S2F42 (HCACK)\n  • 完成 ${r.steps || 0} 個步驟\n\n機台已收到啟動指令，process 開始執行。`};
    }
  },
  {
    name: 'sf-direct',
    pattern: /^[Ss](\d+)[Ff](\d+)/,
    handle: async (match) => {
      const stream = parseInt(match[1]);
      const fn = parseInt(match[2]);
      const r = await apiPost('/send', {stream, function: fn, body: ''});
      if (!r) return {type: 'error', text: `S${stream}F${fn} 送出失敗。`};
      return {type: 'success', text: `${r.sent} → S${stream}F${r.replied} ✓\n\n直接 SECS 訊息送出完成。\n切到 SIMULATOR tab 查看完整 body。`};
    }
  }
];

function matchChatIntent(query) {
  const q = query.toLowerCase().trim();
  for (const intent of CHAT_INTENTS) {
    if (intent.pattern) {
      const m = q.match(intent.pattern);
      if (m) return {intent, match: m};
    }
    if (intent.keywords) {
      for (const kw of intent.keywords) {
        if (q.includes(kw.toLowerCase())) return {intent, match: null};
      }
    }
  }
  return null;
}

function appendChatMsg(role, text) {
  const feed = document.getElementById('chat-feed');
  if (!feed) return null;
  if (feed.querySelector('.empty-state')) feed.innerHTML = '';
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
  feed.appendChild(wrap);
  feed.scrollTop = feed.scrollHeight;
  return bubble;
}

function startThinking(bubble) {
  bubble.classList.add('streaming');
  let dots = 0;
  const id = setInterval(() => {
    dots = (dots + 1) % 4;
    bubble.textContent = '思考中' + '.'.repeat(dots);
  }, 300);
  return () => { clearInterval(id); bubble.classList.remove('streaming'); };
}

async function streamText(bubble, text, speed = 12) {
  bubble.textContent = '';
  bubble.classList.add('streaming');
  for (let i = 0; i < text.length; i++) {
    bubble.textContent += text[i];
    const ch = text[i];
    if (ch !== ' ' && ch !== '\n') {
      const delay = ch === '。' || ch === '\n' ? speed * 4 : speed;
      await new Promise(r => setTimeout(r, delay));
    }
    const feed = document.getElementById('chat-feed');
    if (feed) feed.scrollTop = feed.scrollHeight;
  }
  bubble.classList.remove('streaming');
}

async function chatSend(prefilled) {
  const input = document.getElementById('chat-input');
  const text = (prefilled || input.value).trim();
  if (!text) return;
  appendChatMsg('user', text);
  if (!prefilled) input.value = '';
  input.disabled = true;
  document.getElementById('chat-send-btn').disabled = true;

  const botBubble = appendChatMsg('bot', '');
  const stopThink = startThinking(botBubble);

  await new Promise(r => setTimeout(r, 350 + Math.random() * 400));

  let result;
  const matched = matchChatIntent(text);
  if (!matched) {
    result = {type: 'info', text: '抱歉，我不太懂這個指令。試試輸入「幫助」看我會做什麼，或直接打 SECS 訊息（例如 S1F1）。'};
  } else {
    try {
      result = await matched.intent.handle(matched.match);
    } catch (e) {
      result = {type: 'error', text: '執行失敗：' + (e.message || e)};
    }
  }

  stopThink();
  botBubble.classList.add('chat-' + result.type);
  await streamText(botBubble, result.text);

  input.disabled = false;
  document.getElementById('chat-send-btn').disabled = false;
  input.focus();
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
    btn.addEventListener('click', () => chatSend(btn.dataset.chatSuggest));
  });
});
