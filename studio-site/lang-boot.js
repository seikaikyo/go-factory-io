'use strict';

// HTML 裡寫死的是英文，i18n.js 在 DOMContentLoaded 才把字換成日文或中文。
// 瀏覽器會先把英文畫出來，換字時每段文字寬度都變，整頁跟著位移：
// Speed Insights 量到 CLS 0.405；本機重現時只有中文、日文會出現，英文不會。
// 這支在 <head> 同步執行，存的語言不是英文就先把 body 藏起來，i18n.js 換完字
// 再拿掉 class。i18n.js 沒載到時 3 秒後照樣顯示，寧可看到英文也不要空白頁。
(function () {
  var lang;
  try { lang = localStorage.getItem('studio-lang'); } catch (e) { return; }
  if (lang !== 'ja' && lang !== 'zh') return;
  var root = document.documentElement;
  root.classList.add('i18n-pending');
  setTimeout(function () { root.classList.remove('i18n-pending'); }, 3000);
})();
