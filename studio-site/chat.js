'use strict';

async function call(spec) { return apiPost('/send', spec); }
function ok(text) { return {type: 'success', text}; }
function fail(text) { return {type: 'error', text}; }

const CHAT_INTENTS = [
  // === Meta ===
  {
    name: 'help',
    keywords: ['help', '幫助', '指令', '可以做什麼', '能做什麼', '會什麼', '你會啥'],
    handle: async () => ({
      type: 'info',
      text: '我可以幫你查機台、控制 process、管 recipe、處理警報，30+ 種操作分 6 大類：\n\n  通訊狀態：機台狀況 / 建立連線 / 上線 / 下線\n  身份資訊：機台 ID / 機台資訊 / 狀態變數\n  時間：現在幾點 / 對時\n  警報：有警報嗎 / 確認警報 / 啟用 / 停用警報\n  事件：事件回報 / 定義事件 / 資料追蹤\n  製程：啟動 / 停止 / 暫停 / 繼續 / 中止\n  Recipe：清單 / 目前用哪個 / 切換 / 載入 / 刪除\n  其他：顯示訊息 / spool / 直接 SECS 訊息（如 S1F1）\n\n中文英文都通，用自然語言講就好。'
    })
  },
  {
    name: 'greet',
    keywords: ['你好', '哈囉', '嗨', 'hi', 'hello', '早安', '午安'],
    handle: async () => ({type: 'info', text: '你好，我是 SECSGEM 助理。可以幫你查機台狀況、警報、控制 process、recipe 操作。輸入「幫助」看完整能做什麼。'})
  },

  // === Connectivity ===
  {
    name: 'status',
    keywords: ['機台狀況', '狀況', '狀態', '現在怎樣', '在嗎', '活著', 'status', 'alive'],
    handle: async () => {
      const r = await call({name: 'are_you_there'});
      if (!r) return fail('機台沒回應，HSMS session 可能斷了。先試「建立連線」重連。');
      return ok('機台在線 ✓\n\n  • S1F2 心跳回應正常\n  • Control state：REMOTE（接受 host 指令）\n  • Process state：IDLE，等候啟動');
    }
  },
  {
    name: 'establish',
    keywords: ['建立連線', '連線', 'establish', 'connect', '建立通訊'],
    handle: async () => {
      const r = await call({name: 'establish_comm'});
      if (!r) return fail('S1F13 沒收到回應，HSMS 可能還沒 SELECTED。檢查網路或 port 設定。');
      return ok('通訊建立成功 ✓\n\n  • S1F14 COMMACK = 0（accepted）\n  • Session 已就緒，後續所有 SECS 訊息走這條');
    }
  },
  {
    name: 'online',
    keywords: ['上線', 'online', 'go online', 'request online'],
    handle: async () => {
      const r = await call({name: 'request_online'});
      if (!r) return fail('上線失敗，可能 control state 還在 LOCAL。先確認 operator panel 切到 REMOTE 模式。');
      return ok('機台已上線 ✓\n\n  • S1F18 ONLACK = 0（accepted）\n  • Host 現在可下控制指令\n  • 設備事件會主動 S6F11 上報');
    }
  },
  {
    name: 'offline',
    keywords: ['下線', 'offline', 'request offline'],
    handle: async () => {
      const r = await call({name: 'request_offline'});
      if (!r) return fail('下線指令失敗。');
      return ok('機台已下線 ✓\n\n  • S1F16 OFLACK = 0（accepted）\n  • Host 暫停下控制指令\n  • 設備事件停止主動上報');
    }
  },

  // === Equipment Identity / Status ===
  {
    name: 'equipment-id',
    keywords: ['機台 id', 'eq id', 'equipment id', '設備 id', '識別碼', 'mdln'],
    handle: async () => {
      const r = await call({stream: 1, function: 11, body: ''});
      if (!r) return fail('S1F11 沒回應，可能 SVID 0/1 沒定義或 equipment 不支援。');
      return ok('機台 ID 已查到 ✓\n\n  • S1F12 回傳 MDLN（model）+ SOFTREV（firmware）\n  • SVID 0 = MDLN（機台型號）\n  • SVID 1 = SOFTREV（軟體版本）');
    }
  },
  {
    name: 'equipment-info',
    keywords: ['機台資訊', '機台規格', '硬體資訊', 'equipment info', 'eq info'],
    handle: async () => {
      const r = await call({stream: 1, function: 11, body: ''});
      if (!r) return fail('機台資訊查詢失敗。');
      return ok('已收到機台資訊 ✓\n\n  • S1F12 回傳 model + firmware\n  • 對應 SECS-II 標準 SVID 0（MDLN）/ SVID 1（SOFTREV）\n  • 若要查更多 SVID，用「讀變數」帶 SVID 清單');
    }
  },
  {
    name: 'svreq',
    keywords: ['讀變數', '查狀態變數', 'sv', 'status variable', 'svreq', 'sv 值'],
    handle: async () => {
      const r = await call({stream: 1, function: 3, body: ''});
      if (!r) return fail('S1F3 失敗，SVID 清單可能未定義。');
      return ok('SVREQ 已執行 ✓\n\n  • S1F4 回傳指定 SVID 的當前值\n  • 空 list 代表查全部 SVID\n  • 數值會跟著 process state 變動');
    }
  },
  {
    name: 'collection-event',
    keywords: ['ceid', 'collection event', '收集事件'],
    handle: async () => {
      const r = await call({stream: 2, function: 33, body: ''});
      if (!r) return fail('S2F33 失敗。');
      return ok('CEID 訂閱定義已送出 ✓\n\n  • S2F34 DRACK = 0（accepted）\n  • RPTID 與 CEID 綁定關係已更新\n  • 後續對應 event 觸發時會 S6F11 上報');
    }
  },

  // === Clock ===
  {
    name: 'clock-get',
    keywords: ['現在幾點', '查時間', 'clock', '幾點了', 'time'],
    handle: async () => {
      const r = await call({stream: 2, function: 17, body: ''});
      if (!r) return fail('S2F17 沒回應，equipment 可能不支援 clock query。');
      return ok('機台時間查詢完成 ✓\n\n  • S2F18 回傳 RTC（YYYYMMDDhhmmss 格式）\n  • 與 host 時間有差異建議「對時」同步\n  • drift 太大會影響事件 timestamp 對齊');
    }
  },
  {
    name: 'clock-set',
    keywords: ['對時', '同步時間', 'sync time', 'sync clock', '校時'],
    handle: async () => {
      const r = await call({stream: 2, function: 31, body: ''});
      if (!r) return fail('對時失敗，equipment 可能拒絕 clock write。');
      return ok('時間同步完成 ✓\n\n  • S2F32 TIACK = 0（accepted）\n  • Equipment RTC 已寫入 host 當前時間\n  • drift 歸零，事件 timestamp 對齊');
    }
  },

  // === Alarms ===
  {
    name: 'alarms',
    keywords: ['警報', 'alarm', '異常', '報警', '錯誤', '有錯嗎', '有事嗎', '問題'],
    handle: async () => {
      const r = await call({name: 'alarm_report'});
      if (!r) return fail('警報查詢失敗。');
      return ok('警報回報已觸發 ✓\n\n  • S5F1 上報 ALID + ALCD（severity）+ ALTX（alarm text）\n  • S5F2 ACK（host 已收）\n  • 若 ALCD bit 7 = 1 代表 alarm SET，= 0 代表 alarm CLEAR');
    }
  },
  {
    name: 'enable-alarm',
    keywords: ['啟用警報', 'enable alarm', '開啟警報'],
    handle: async () => {
      const r = await call({stream: 5, function: 3, body: ''});
      if (!r) return fail('S5F3 啟用警報失敗。');
      return ok('警報已啟用 ✓\n\n  • S5F4 ACKC5 = 0（accepted）\n  • 指定 ALID 的 enable bit ON\n  • 未來 alarm 觸發會主動 S5F1 上報');
    }
  },
  {
    name: 'disable-alarm',
    keywords: ['停用警報', 'disable alarm', '關閉警報'],
    handle: async () => {
      const r = await call({stream: 5, function: 3, body: ''});
      if (!r) return fail('S5F3 停用警報失敗。');
      return ok('警報已停用 ✓\n\n  • S5F4 ACKC5 = 0（accepted）\n  • 指定 ALID 的 enable bit OFF\n  • Alarm 仍會發生但不會主動上報，避免雜訊');
    }
  },
  {
    name: 'ack-alarm',
    keywords: ['確認警報', 'ack alarm', 'clear alarm', '清除警報', '清警報'],
    handle: async () => {
      const r = await call({stream: 5, function: 5, body: ''});
      if (!r) return fail('S5F5 警報清單查詢失敗。');
      return ok('Alarm 清單已取得 ✓\n\n  • S5F6 回傳所有 enabled ALID 與 ALCD/ALTX\n  • 可從清單挑要 ack 的 alarm 後續處理\n  • 設備警報邏輯見 SEMI E5 specification');
    }
  },

  // === Events ===
  {
    name: 'events',
    keywords: ['事件', 'event', '回報', 'event report'],
    handle: async () => {
      const r = await call({name: 'event_report'});
      if (!r) return fail('事件回報失敗。');
      return ok('事件回報觸發完成 ✓\n\n  • S6F11 上報 CEID + RPTID + 對應 SV/DV 數值\n  • S6F12 ACKC6 = 0（host 已收）\n  • CEID 對應的事件條件已滿足，host 端 SECS 解析器處理資料');
    }
  },
  {
    name: 'event-define',
    keywords: ['定義事件', 'define event', 'event define', 'report def'],
    handle: async () => {
      const r = await call({stream: 2, function: 37, body: ''});
      if (!r) return fail('S2F37 event definition 失敗。');
      return ok('Event report 定義完成 ✓\n\n  • S2F38 ERACK = 0（accepted）\n  • 已建立 RPTID → CEID 的綁定關係\n  • 後續對應 collection event 觸發會用此 RPTID 上報');
    }
  },
  {
    name: 'data-trace',
    keywords: ['資料追蹤', 'trace data', '資料收集', 'data collect', 'trace'],
    handle: async () => {
      const r = await call({stream: 6, function: 1, body: ''});
      if (!r) return fail('S6F1 trace data 失敗。');
      return ok('Trace data 已取樣 ✓\n\n  • S6F2 ACKC6 = 0（host 已收）\n  • 取樣間隔與 SVID 範圍由 S2F23 trace initialize 設定\n  • 用來做 process variable 時序分析');
    }
  },

  // === Process Control ===
  {
    name: 'start',
    keywords: ['啟動', '開始', 'start', 'run', 'rcmd start'],
    handle: async () => {
      const r = await call({name: 'rcmd_start'});
      if (!r) return fail('RCMD START 失敗，可能 process state 不在 IDLE。');
      return ok('RCMD START 已執行 ✓\n\n  • S2F42 HCACK = 0（accepted）\n  • Process state IDLE → EXECUTING\n  • 接著會看到 S6F11 ProcessStart event 上報');
    }
  },
  {
    name: 'stop',
    keywords: ['停止', '停下來', 'stop', 'rcmd stop', '停機'],
    handle: async () => {
      const r = await call({stream: 2, function: 41, body: 'L:2 { A "STOP" L:0 }'});
      if (!r) return fail('RCMD STOP 失敗。');
      return ok('RCMD STOP 已執行 ✓\n\n  • S2F42 HCACK = 0（accepted）\n  • 等待當前 step 跑完 → process state 切到 IDLE\n  • 不像 ABORT，stop 會等乾淨點再停');
    }
  },
  {
    name: 'pause',
    keywords: ['暫停', 'pause', 'rcmd pause'],
    handle: async () => {
      const r = await call({stream: 2, function: 41, body: 'L:2 { A "PAUSE" L:0 }'});
      if (!r) return fail('RCMD PAUSE 失敗，process state 可能不在 EXECUTING。');
      return ok('RCMD PAUSE 已執行 ✓\n\n  • S2F42 HCACK = 0（accepted）\n  • Process state EXECUTING → PAUSE\n  • 當前 step 狀態保留，後續 RESUME 從相同位置接回');
    }
  },
  {
    name: 'resume',
    keywords: ['繼續', '恢復', 'resume', 'rcmd resume'],
    handle: async () => {
      const r = await call({stream: 2, function: 41, body: 'L:2 { A "RESUME" L:0 }'});
      if (!r) return fail('RCMD RESUME 失敗，process state 可能不在 PAUSE。');
      return ok('RCMD RESUME 已執行 ✓\n\n  • S2F42 HCACK = 0（accepted）\n  • Process state PAUSE → EXECUTING\n  • 從 pause 點繼續未跑完的 step，不重跑');
    }
  },
  {
    name: 'abort',
    keywords: ['中止', 'abort', 'rcmd abort', '取消'],
    handle: async () => {
      const r = await call({stream: 2, function: 41, body: 'L:2 { A "ABORT" L:0 }'});
      if (!r) return fail('RCMD ABORT 失敗。');
      return ok('RCMD ABORT 已執行 ✓\n\n  • S2F42 HCACK = 0（accepted）\n  • Process 強制中止，狀態不保留\n  • 需要 host 重設參數 + 重 START 才能跑下一輪');
    }
  },

  // === Recipe (Process Program) ===
  {
    name: 'recipe-list',
    keywords: ['有哪些 recipe', 'recipe 清單', 'recipe list', 'pp list', 'ppdir', '所有 recipe'],
    handle: async () => {
      const r = await call({stream: 7, function: 19, body: ''});
      if (!r) return fail('PPDIR 查詢失敗。');
      return ok('Recipe 清單已取得 ✓\n\n  • S7F20 回傳設備所有已存在的 PPID\n  • 數量 + ID 因設備而異\n  • 要看單一 recipe 內容用「目前 recipe」或「載入 recipe」');
    }
  },
  {
    name: 'recipe-current',
    keywords: ['目前 recipe', '現在用什麼 recipe', 'current recipe', '正在跑哪個 recipe'],
    handle: async () => {
      const r = await call({stream: 7, function: 25, body: ''});
      if (!r) return fail('Recipe verify 失敗。');
      return ok('目前 recipe 已查到 ✓\n\n  • S7F26 回傳 active PPID + 內容雜湊\n  • 雜湊用來驗 host/equipment 兩邊 recipe 一致\n  • 若雜湊不一致建議重新「載入 recipe」覆蓋');
    }
  },
  {
    name: 'recipe-select',
    keywords: ['選 recipe', 'select recipe', 'pp select', '切換 recipe'],
    handle: async () => {
      const r = await call({stream: 7, function: 1, body: ''});
      if (!r) return fail('PPSelect 失敗。');
      return ok('Recipe 切換 inquire 完成 ✓\n\n  • S7F2 PPGNT = 0（granted），equipment 同意載入\n  • 若 PPGNT ≠ 0 代表設備記憶體不足或 PPID 已存在衝突\n  • 接著用「載入 recipe」實際把內容寫進去');
    }
  },
  {
    name: 'recipe-load',
    keywords: ['載入 recipe', 'load recipe', 'pp load', '上傳 recipe'],
    handle: async () => {
      const r = await call({stream: 7, function: 23, body: ''});
      if (!r) return fail('Recipe load 失敗。');
      return ok('Recipe 載入完成 ✓\n\n  • S7F24 ACKC7 = 0（accepted）\n  • Recipe 內容已寫入 equipment PP storage\n  • 接著可用「切換 recipe」設成 active');
    }
  },
  {
    name: 'recipe-delete',
    keywords: ['刪除 recipe', 'delete recipe', 'pp delete', '砍 recipe'],
    handle: async () => {
      const r = await call({stream: 7, function: 17, body: ''});
      if (!r) return fail('Recipe delete 失敗。');
      return ok('Recipe 已刪除 ✓\n\n  • S7F18 ACKC7 = 0（accepted）\n  • 指定 PPID 已從 equipment storage 移除\n  • 若該 PPID 是 active recipe，equipment 會自動切到 default');
    }
  },

  // === Other ===
  {
    name: 'terminal-msg',
    keywords: ['顯示訊息', '傳訊息', 'terminal message', 'show message', '操作員訊息'],
    handle: async () => {
      const r = await call({stream: 10, function: 3, body: ''});
      if (!r) return fail('Terminal display 失敗。');
      return ok('訊息已送到操作員終端 ✓\n\n  • S10F4 ACK10 = 0（accepted）\n  • 文字已顯示在 equipment 操作介面\n  • 用於提示 operator 注意事項或下一步動作');
    }
  },
  {
    name: 'spool-toggle',
    keywords: ['spool', 'spool data', '開啟 spool', '關閉 spool', 'spool 控制'],
    handle: async () => {
      const r = await call({stream: 6, function: 23, body: ''});
      if (!r) return fail('Spool data request 失敗。');
      return ok('Spool 資料拉取完成 ✓\n\n  • S6F24 spool 暫存的事件已回傳\n  • 用於斷線期間 equipment 累積的 S6F11 event 補抓\n  • 拉完後 equipment spool buffer 會清空');
    }
  },

  // === Catch-all (regex) ===
  {
    name: 'sf-direct',
    pattern: /^[Ss](\d+)[Ff](\d+)/,
    handle: async (match) => {
      const stream = parseInt(match[1]);
      const fn = parseInt(match[2]);
      const r = await call({stream, function: fn, body: ''});
      if (!r) return fail(`S${stream}F${fn} 送出失敗。`);
      const reply = r.replied != null ? r.replied : (fn + 1);
      return ok(`S${stream}F${fn} 已送出 ✓\n\n  • Equipment 回 S${stream}F${reply}\n  • 原始 SECS 訊息直送，未經意圖層解析\n  • 適合除錯特定 stream/function 行為`);
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
  bubble.scrollIntoView({block: 'nearest'});
  return bubble;
}

function startThinking(bubble) {
  bubble.classList.add('thinking');
  bubble.textContent = '思考中';
  let dots = 0;
  const id = setInterval(() => {
    dots = (dots + 1) % 4;
    bubble.textContent = '思考中' + '.'.repeat(dots);
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
      const delay = ch === '。' ? speed * 4 : speed;
      await new Promise(r => setTimeout(r, delay));
    }
    if (i % 5 === 0) bubble.scrollIntoView({block: 'nearest'});
  }
  bubble.scrollIntoView({block: 'nearest'});
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

  const botBubble = appendChatMsg('bot', '思考中');
  const stopThink = startThinking(botBubble);

  await new Promise(r => setTimeout(r, 350 + Math.random() * 400));

  let result;
  const matched = matchChatIntent(text);
  if (!matched) {
    result = {type: 'info', text: '抱歉，我不太懂這個指令。輸入「幫助」看完整 30+ 種能做的事，或直接打 SECS 訊息（例如 S1F1）。'};
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
