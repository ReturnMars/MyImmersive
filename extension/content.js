console.log("[MyImmersive] Content Script Loaded");

// 注入全局样式
const style = document.createElement('style');
style.textContent = `
  .my-trans-container {
    color: #5d6d7e !important; /* 优雅的蓝灰色，比纯黑淡一些 */
    font-size: 0.95em !important;
    font-family: inherit !important;
    /* font-style: italic !important; */ /* 用户偏好：取消斜体 */
    margin: 6px 0 10px 0 !important;
    padding: 2px 0 2px 12px !important;
    background: transparent !important;
    border-left: 2px solid #68d391 !important; /* 保持品牌的淡绿色窄线作为标记 */
    line-height: 1.6 !important;
    display: block !important;
    width: 100% !important;
    word-break: break-word !important;
    opacity: 0.9 !important;
  }
  .my-trans-loading {
    border-bottom: 1px dashed rgba(104, 211, 145, 0.5) !important;
    transition: all 0.3s;
  }
  #my-trans-control {
    position: fixed;
    bottom: 30px;
    right: 30px;
    z-index: 2147483647; /* 极高层级，确保在 GitHub 等页面之上 */
    padding: 10px 20px;
    border-radius: 25px;
    background: #2c7a7b;
    color: white;
    font-size: 14px;
    font-weight: bold;
    cursor: pointer;
    box-shadow: 0 4px 15px rgba(0,0,0,0.3);
    border: none;
    transition: all 0.3s;
    user-select: none;
    display: flex;
    align-items: center;
    gap: 8px;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
  }
  #my-trans-control:hover {
    transform: scale(1.05);
    background: #285e5e;
  }
  #my-trans-control.active {
    background: #e53e3e;
  }
  #my-trans-status-text {
    font-size: 11px;
    opacity: 0.8;
    font-weight: normal;
  }
`;
document.head.appendChild(style);

// 创建控制按钮
const controlBtn = document.createElement('button');
controlBtn.id = 'my-trans-control';
controlBtn.innerHTML = `<span>开始翻译</span><span id="my-trans-status-text"></span>`;
document.body.appendChild(controlBtn);

let isTranslating = false;
let abortController = null;
let concurrentLimit = 4; // 并行任务数

function updateBtnStatus(mainText, subText = "", isActive = false) {
  const mainSpan = controlBtn.querySelector('span');
  const subSpan = controlBtn.querySelector('#my-trans-status-text');
  if (mainSpan) mainSpan.innerText = mainText;
  if (subSpan) subSpan.innerText = subText;
  if (isActive) {
    controlBtn.classList.add('active');
  } else {
    controlBtn.classList.remove('active');
  }
}

async function startTranslation() {
  if (isTranslating) {
    stopTranslation();
    return;
  }

  isTranslating = true;
  updateBtnStatus("停止翻译", "扫描中...", true);
  abortController = new AbortController();

  const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT, null, false);
  let node;
  const allSegments = [];
  const allNodes = [];

  const allowTags = ["P", "LI", "DIV", "SPAN", "H1", "H2", "H3", "H4", "H5", "H6"];
  const ignoreTags = ["SCRIPT", "STYLE", "PRE", "CODE", "NOSCRIPT", "INPUT", "TEXTAREA", "BUTTON", "A", "NAV", "FOOTER"];

  while ((node = walker.nextNode())) {
    const parent = node.parentElement;
    if (!parent) continue;
    if (parent.closest('.my-trans-container') || parent.id === 'my-trans-status-text' || parent.id === 'my-trans-control') continue;
    if (parent.classList.contains('my-trans-loading') || parent.querySelector('.my-trans-container')) continue;
    
    if (ignoreTags.includes(parent.tagName)) continue;
    
    const text = node.textContent.trim();
    if (text.length > 20 && allowTags.includes(parent.tagName)) {
      allSegments.push(text);
      allNodes.push(node);
      parent.classList.add('my-trans-loading');
    }
  }

  const total = allSegments.length;
  if (total === 0) {
    isTranslating = false;
    updateBtnStatus("开始翻译", "未发现内容");
    setTimeout(() => updateBtnStatus("开始翻译"), 3000);
    return;
  }

  const BATCH_SIZE = 15; // 减小批次大小，通过并行来提速，增加“长出来”的频率
  const batches = [];
  for (let i = 0; i < total; i += BATCH_SIZE) {
    batches.push({
      segments: allSegments.slice(i, i + BATCH_SIZE),
      nodes: allNodes.slice(i, i + BATCH_SIZE),
      id: i
    });
  }

  let finishedCount = 0;
  console.log(`[MyImmersive] Total batches: ${batches.length}, Parallelism: ${concurrentLimit}`);

  // 并行执行池
  async function worker() {
    while (batches.length > 0 && isTranslating) {
      const batch = batches.shift();
      try {
        await processBatch(batch);
        finishedCount += batch.segments.length;
        updateBtnStatus("停止翻译", `${Math.min(finishedCount, total)} / ${total}`, true);
      } catch (err) {
        console.error(`[MyImmersive] Worker batch error:`, err);
      }
    }
  }

  async function processBatch(batch) {
    const { segments, nodes, id } = batch;
    const response = await fetch("http://localhost:8080/api/translate", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ url: window.location.href, segments: segments }),
      signal: abortController.signal
    });

    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const data = await response.json();

    if (data.translations) {
      data.translations.forEach((trans, idx) => {
        const tNode = nodes[idx];
        if (tNode && tNode.parentNode) {
          const p = tNode.parentElement;
          p.classList.remove('my-trans-loading');
          if (p.querySelector('.my-trans-container')) return;

          const d = document.createElement("div");
          d.className = "my-trans-container";
          d.innerText = trans;
          try { tNode.after(d); } catch (e) { p.appendChild(d); }
        }
      });
    }
  }

  // 启动并行工作器
  const workers = Array(Math.min(concurrentLimit, batches.length)).fill(worker);
  await Promise.all(workers.map(w => w()));

  isTranslating = false;
  updateBtnStatus("开始翻译", finishedCount >= total ? "翻译完成" : "已停止");
  setTimeout(() => updateBtnStatus("开始翻译"), 5000);
}

function stopTranslation() {
  isTranslating = false;
  if (abortController) {
    abortController.abort();
  }
  document.querySelectorAll('.my-trans-loading').forEach(el => el.classList.remove('my-trans-loading'));
  updateBtnStatus("开始翻译", "已停止");
  setTimeout(() => updateBtnStatus("开始翻译"), 2000);
}

controlBtn.addEventListener('click', startTranslation);
