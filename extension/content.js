console.log("[MyImmersive] Content Script Loaded");

// 样式已通过 manifest.json 的 css 配置注入，无需手动创建

// 配置
let CONFIG = {
  targetLang: "zh-CN",
  apiUrl: "http://localhost:8080",
  autoTranslate: false,
};

// 加载配置
async function loadConfig() {
  try {
    const result = await chrome.storage.sync.get(CONFIG);
    CONFIG = { ...CONFIG, ...result };
    console.log("[MyImmersive] Config loaded:", CONFIG);
  } catch (err) {
    console.error("[MyImmersive] Failed to load config:", err);
  }
}

// 创建控制按钮
const controlBtn = document.createElement("button");
controlBtn.id = "my-trans-control";
controlBtn.innerHTML = `<span>开始翻译</span><span id="my-trans-status-text"></span>`;
document.body.appendChild(controlBtn);

let isTranslating = false;
let abortController = null;
let concurrentLimit = 4; // 并行任务数

function updateBtnStatus(mainText, subText = "", isActive = false) {
  const mainSpan = controlBtn.querySelector("span");
  const subSpan = controlBtn.querySelector("#my-trans-status-text");
  if (mainSpan) mainSpan.innerText = mainText;
  if (subSpan) subSpan.innerText = subText;
  if (isActive) {
    controlBtn.classList.add("active");
  } else {
    controlBtn.classList.remove("active");
  }
}

// 错误提示 Toast
function showToast(message, type = "error") {
  // 移除已存在的 toast
  const existing = document.getElementById("my-trans-toast");
  if (existing) existing.remove();

  const toast = document.createElement("div");
  toast.id = "my-trans-toast";
  toast.className = `my-trans-toast my-trans-toast-${type}`;
  toast.textContent = message;
  document.body.appendChild(toast);

  // 3秒后自动消失
  setTimeout(() => {
    toast.classList.add("my-trans-toast-hide");
    setTimeout(() => toast.remove(), 300);
  }, 3000);
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
  const blockTags = [
    "p",
    "li",
    "h1",
    "h2",
    "h3",
    "h4",
    "h5",
    "h6",
    "td",
    "th",
    "blockquote",
  ];
  const selector = blockTags.join(",");

  const allElements = [];
  const allSegments = [];
  const allPlaceholders = []; // 存储每个元素的占位符映射

  // 提取并替换内嵌标签为占位符 (安全模式：忽略 script/style 等节点)
  function extractWithPlaceholders(element) {
    const placeholders = [];
    const inlineTags = ["code", "strong", "em", "b", "i", "mark"];

    // 克隆元素以防修改原 DOM
    const clone = element.cloneNode(true);

    // 1. 彻底移除所有 script 和 style 标签，防止 JSON 或 CSS 被提取进行翻译
    clone.querySelectorAll("script, style").forEach((s) => s.remove());

    let html = clone.innerHTML;

    inlineTags.forEach((tag) => {
      // 匹配标签，保留完整标签 (class 等属性也保留)
      const regex = new RegExp(`<${tag}[^>]*>([\\s\\S]*?)</${tag}>`, "gi");
      html = html.replace(regex, (match, content) => {
        const idx = placeholders.length;
        placeholders.push({ original: match, content: content.trim() });
        return `{{${idx}}}`;
      });
    });

    // 2. 清理剩余的 HTML 标签，只保留文本和占位符
    let text = html.replace(/<[^>]+>/g, " ").replace(/\s+/g, " ");
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

  document.querySelectorAll(selector).forEach((el) => {
    // 跳过已处理或特殊元素
    if (el.classList.contains("my-trans-origin")) return;
    if (el.classList.contains("my-trans-translated")) return;
    if (el.classList.contains("my-trans-loading")) return;
    if (el.closest(".my-trans-container")) return;
    if (el.querySelector(".my-trans-container")) return;
    if (el.closest("#my-trans-control")) return;

    // 跳过嵌套在忽略标签或代码区域内的元素
    const skipSelectors = [
      "nav",
      "footer",
      "pre",
      "script",
      "style",
      "noscript",
      "button",
      ".blob-wrapper",
      ".blob-code",
      ".blob-code-inner",
      ".js-file-line-container", // GitHub 代码区
      ".react-directory-filename-column",
      ".react-directory-truncate", // GitHub 文件名列
      "td.age",
      "td.message + td", // GitHub 日期列
      ".highlight",
      ".codehiliting",
      ".syntax",
      ".gist", // 通用高亮
      '[class*="hljs"]',
      '[class*="language-"]',
      '[class*="prism"]', // 库特定
    ];
    if (el.closest(skipSelectors.join(","))) return;

    // 提取并处理内嵌标签
    const { text, placeholders } = extractWithPlaceholders(el);
    if (text.trim().length > 2) {
      allElements.push(el);
      allSegments.push(text.trim());
      allPlaceholders.push(placeholders);
      el.classList.add("my-trans-loading");
    }
  });

  const total = allElements.length;
  if (total === 0) {
    isTranslating = false;
    updateBtnStatus("开始翻译", "未发现新内容");
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
      id: i,
    });
  }

  let finishedCount = 0;
  console.log(
    `[MyImmersive] Total blocks: ${total}, Batches: ${batches.length}, Parallelism: ${concurrentLimit}`,
  );

  // 并行执行池
  async function worker() {
    while (batches.length > 0 && isTranslating) {
      const batch = batches.shift();
      try {
        await processBatch(batch);
        finishedCount += batch.segments.length;
        updateBtnStatus(
          "停止翻译",
          `${Math.min(finishedCount, total)} / ${total}`,
          true,
        );
      } catch (err) {
        console.error(`[MyImmersive] Worker batch error:`, err);

        // 错误分类处理
        if (err.name === "AbortError") {
          // 用户主动停止，不显示错误
        } else if (
          err.message.includes("Failed to fetch") ||
          err.message.includes("NetworkError")
        ) {
          showToast("❌ 网络连接失败，请检查后端服务", "error");
        } else if (
          err.message.includes("429") ||
          err.message.includes("rate limit")
        ) {
          showToast("⚠️ API 请求频繁，请稍后重试", "warning");
        } else if (err.message.includes("401") || err.message.includes("403")) {
          showToast("❌ API Key 无效或已过期", "error");
        } else if (
          err.message.includes("500") ||
          err.message.includes("502") ||
          err.message.includes("503")
        ) {
          showToast("❌ 服务器错误，请稍后重试", "error");
        } else {
          showToast(`❌ 翻译失败: ${err.message}`, "error");
        }
      }
    }
  }

  async function processBatch(batch) {
    const { segments, elements, placeholders } = batch;
    const response = await fetch(`${CONFIG.apiUrl}/api/translate`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ url: window.location.href, segments: segments }),
      signal: abortController.signal,
    });

    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const data = await response.json();

    if (data.translations) {
      data.translations.forEach((trans, idx) => {
        const el = elements[idx];
        const phs = placeholders[idx];
        if (el) {
          el.classList.remove("my-trans-loading");

          const isTableValue = el.tagName === "TD" || el.tagName === "TH";

          // 检查是否已经插入过翻译内容
          // 如果是表格单元格，检查子元素；否则检查下一个兄弟节点
          const alreadyTranslated = isTableValue
            ? el.querySelector(":scope > .my-trans-translated")
            : el.nextElementSibling &&
              el.nextElementSibling.classList.contains("my-trans-translated");

          if (alreadyTranslated) {
            el.classList.add("my-trans-origin");
            return;
          }

          // 创建翻译 DOM
          // 对于表格单元格，使用 div 作为包装以防止破坏表格布局
          const d = document.createElement(isTableValue ? "div" : el.tagName);

          // 复制原始元素的类名以复用样式
          d.className = el.className;
          d.classList.add("my-trans-translated");
          d.classList.remove("my-trans-loading", "my-trans-origin");

          // 还原占位符为原始标签
          if (phs && phs.length > 0) {
            d.innerHTML = restorePlaceholders(trans, phs);
          } else {
            d.innerText = trans;
          }

          if (isTableValue) {
            el.appendChild(d);
          } else {
            el.after(d);
          }
          el.classList.add("my-trans-origin");
        }
      });
    }
  }

  // 启动并行工作器
  const workers = Array(Math.min(concurrentLimit, batches.length)).fill(worker);
  await Promise.all(workers.map((w) => w()));

  isTranslating = false;
  updateBtnStatus("开始翻译", finishedCount >= total ? "翻译完成" : "已停止");
  setTimeout(() => updateBtnStatus("开始翻译"), 5000);
}

function stopTranslation() {
  isTranslating = false;
  if (abortController) {
    abortController.abort();
  }
  document
    .querySelectorAll(".my-trans-loading")
    .forEach((el) => el.classList.remove("my-trans-loading"));
  updateBtnStatus("开始翻译", "已停止");
  setTimeout(() => updateBtnStatus("开始翻译"), 2000);
}

controlBtn.addEventListener("click", startTranslation);

// 初始化
async function init() {
  await loadConfig();

  // 如果开启了自动翻译，页面加载后自动开始
  if (CONFIG.autoTranslate) {
    console.log("[MyImmersive] Auto-translate enabled, starting...");
    setTimeout(startTranslation, 1000); // 等待页面稳定
  }
}

init();
