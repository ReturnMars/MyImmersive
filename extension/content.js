console.log("[MyImmersive] Content Script Loaded");

// 样式已通过 manifest.json 的 css 配置注入，无需手动创建

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

  // 改为以块级元素为单位，而非文本节点
  const blockTags = ["p", "li", "h1", "h2", "h3", "h4", "h5", "h6", "td", "th", "blockquote"];
  const selector = blockTags.join(",");
  
  const allElements = [];
  const allSegments = [];
  const allPlaceholders = []; // 存储每个元素的占位符映射

  // 提取并替换内嵌标签为占位符
  function extractWithPlaceholders(html) {
    const placeholders = [];
    const inlineTags = ['code', 'strong', 'em', 'b', 'i', 'mark'];
    let text = html;
    
    inlineTags.forEach(tag => {
      // 匹配标签，保留完整标签 (class 等属性也保留)
      const regex = new RegExp(`<${tag}[^>]*>([\\s\\S]*?)</${tag}>`, 'gi');
      text = text.replace(regex, (match, content) => {
        const idx = placeholders.length;
        placeholders.push({ original: match, content: content.trim() });
        return `{{${idx}}}`;
      });
    });
    
    // 清理其他 HTML 标签，只保留文本和占位符
    text = text.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ');
    return { text, placeholders };
  }

  // 还原占位符为原始标签
  function restorePlaceholders(translated, placeholders) {
    let result = translated;
    placeholders.forEach((p, idx) => {
      result = result.replace(`{{${idx}}}`, p.original);
    });
    return result;
  }

  document.querySelectorAll(selector).forEach(el => {
    // 跳过已处理或特殊元素
    if (el.closest('.my-trans-container')) return;
    if (el.querySelector('.my-trans-container')) return;
    if (el.classList.contains('my-trans-loading')) return;
    if (el.closest('#my-trans-control')) return;
    
    // 跳过嵌套在忽略标签内的元素
    if (el.closest('nav, footer, pre, script, style, noscript, button')) return;
    
    // 提取并处理内嵌标签
    const { text, placeholders } = extractWithPlaceholders(el.innerHTML);
    if (text.trim().length > 10) {
      allElements.push(el);
      allSegments.push(text.trim());
      allPlaceholders.push(placeholders);
      el.classList.add('my-trans-loading');
    }
  });

  const total = allElements.length;
  if (total === 0) {
    isTranslating = false;
    updateBtnStatus("开始翻译", "未发现内容");
    setTimeout(() => updateBtnStatus("开始翻译"), 3000);
    return;
  }

  const BATCH_SIZE = 15;
  const batches = [];
  for (let i = 0; i < total; i += BATCH_SIZE) {
    batches.push({
      segments: allSegments.slice(i, i + BATCH_SIZE),
      elements: allElements.slice(i, i + BATCH_SIZE),
      placeholders: allPlaceholders.slice(i, i + BATCH_SIZE),
      id: i
    });
  }

  let finishedCount = 0;
  console.log(`[MyImmersive] Total blocks: ${total}, Batches: ${batches.length}, Parallelism: ${concurrentLimit}`);

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
    const { segments, elements, placeholders } = batch;
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
        const el = elements[idx];
        const phs = placeholders[idx];
        if (el) {
          el.classList.remove('my-trans-loading');
          if (el.querySelector('.my-trans-translated')) return;

          const d = document.createElement(el.tagName);
          d.className = el.className;
          d.classList.add('my-trans-translated');
          d.classList.remove('my-trans-loading');
          
          // 还原占位符为原始标签
          if (phs && phs.length > 0) {
            d.innerHTML = restorePlaceholders(trans, phs);
          } else {
            d.innerText = trans;
          }
          
          el.after(d);
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
