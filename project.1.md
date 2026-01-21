既然决定放弃本地部署，转而使用 **AI API**，那么整个架构会变得更加**轻量化**和**通用化**。你的笔记本不再承担计算压力，Golang 后端将主要充当**流量调度、缓存管理和 Prompt 注入**的角色。

以下是为你量身定制的**“自制沉浸式翻译（API版）”完整项目方案**。

---

### 一、 项目概览

*   **项目名称**：MyImmersive (Go-Backend Edition)
*   **核心目标**：实现网页双语对照翻译，重点优化 **GitHub/技术文档** 场景（保留术语、不乱翻代码、识别上下文）。
*   **架构模式**：
    *   **Frontend (Chrome 插件)**：负责“搬运”。提取网页文本 -> 发送给后端 -> 接收结果 -> 渲染上屏。
    *   **Backend (Golang)**：负责“思考”。管理 API Key、构建智能 Prompt、并发控制、本地缓存。
    *   **AI Provider**：**DeepSeek V3** (推荐) 或 **Google Gemini Flash**。

---

### 二、 技术选型与成本预算

#### 1. AI API 选择 (推荐 DeepSeek)
*   **理由**：目前中文语境下性价比之王。不仅翻译准确，而且对程序员术语（Code-aware）理解极佳。
*   **成本**：DeepSeek-V3 的价格约为 **1元人民币 / 百万 tokens**。
    *   **概念**：翻译一本几十万字的技术书可能只需要几毛钱。个人高强度使用，一个月也很难超过 10 块钱。
*   **备选**：**Google Gemini 1.5 Flash**（有免费层级，速度极快，但中文地道程度略逊于 DeepSeek）。

#### 2. 后端技术栈
*   **语言**：Golang 1.25+
*   **Web框架**：Gin (轻量、高性能) 或 Standard Lib (足够简单)。
*   **缓存库**：**BadgerDB** (纯 Go 编写的 KV 存储，无需安装 Redis) 或 **Go-Cache** (内存缓存)。

#### 3. 前端技术栈
*   **规范**：Manifest V3
*   **构建**：原生 JavaScript (无需 React/Vue，减少复杂度)。

---

### 三、 详细实施方案

#### 1. 核心流程设计

1.  **解析 (Frontend)**：插件通过 `TreeWalker` 扫描网页，提取 `P`, `H1-H6`, `LI` 等块级元素，**强力过滤** `<pre>`, `<code>` 等代码容器。
2.  **打包 (Frontend)**：将提取到的 10~20 个段落打包成一个 JSON，附带当前网页 URL。
3.  **查询缓存 (Backend)**：Go 服务计算 `MD5(URL + Text)`，查 BadgerDB。如果命中，直接返回。
4.  **智能请求 (Backend)**：未命中缓存，构建 System Prompt，调用 DeepSeek API。
5.  **回填 (Backend -> Frontend)**：将结果存入缓存并返回前端。
6.  **渲染 (Frontend)**：在原文节点下方插入 `<div class="my-trans">...</div>`。

#### 2. 关键代码逻辑 (Go Backend)

这是一个处理 API 交互和 Prompt 的核心结构：

```go
// 伪代码逻辑展示
type TransRequest struct {
    URL      string   `json:"url"`      // 用于上下文判断
    Segments []string `json:"segments"` // 待翻译文本段落
}

func handleTranslate(c *gin.Context) {
    var req TransRequest
    c.BindJSON(&req)

    // 1. 检查缓存 (略)
    
    // 2. 构建 Prompt (核心)
    systemPrompt := "你是一个翻译引擎。"
    
    // 如果是 GitHub 或 技术站点，注入强化指令
    if strings.Contains(req.URL, "github.com") || strings.Contains(req.URL, "stackoverflow") {
        systemPrompt = `你是一个资深全栈工程师。请将文本翻译为中文。
        规则：
        1. 遇到代码变量（camelCase, snake_case）绝对保留原样。
        2. 保留技术术语（如 Commit, Pull Request, Repo, Deploy）。
        3. 不要输出任何Markdown格式，直接返回纯文本翻译。`
    }

    // 3. 调用 AI API (使用 go-openai 库或 resty)
    // 注意：这里要实现并发控制，不要把 API QPS 打爆
    translatedTexts := callDeepSeekAPI(systemPrompt, req.Segments)

    // 4. 写入缓存并返回
    c.JSON(200, gin.H{"translations": translatedTexts})
}
```

#### 3. GitHub 场景深度优化策略

为了解决你提到的“人名、代码块、技术名词”问题，我们需要在三个层面拦截：

*   **L1：DOM 物理隔离 (Frontend)**
    *   在 JS 中，如果一个文本节点的父级链中包含 `.blob-code`, `.diff-table`, `pre`，直接跳过。
    *   **正则过滤**：如果一段文本看起来像代码（例如包含 `{ }`, `func `, `var ` 且长度很短），不发送给后端。

*   **L2：Prompt 语义隔离 (Backend)**
    *   利用 API 的 `user` 和 `system` 角色。
    *   将 URL 发给 LLM，让它知道：“我在看 Vue.js 的源码文档”，它会自动调整术语库。

*   **L3：后处理清洗 (Backend)**
    *   Go 收到 API 返回的翻译后，做一次正则检查。如果发现翻译结果把 `fmt.Println` 变成了 `格式化.打印行`，直接丢弃该翻译，返回原文或空字符串，防止误导。

---

### 四、 遇到的问题预判与解决方案

| 问题 | 现象 | 解决方案 |
| :--- | :--- | :--- |
| **API 限流 (429 Error)** | 页面段落太多，瞬间发起几十个请求，API 拒绝服务。 | **Go 信号量 (Semaphore)**：限制最大并发数为 5。前端做去抖动 (Debounce)，滚动到底部再触发下一批翻译。 |
| **翻译慢 (Latency)** | API 响应通常需要 1-3 秒，页面看起来像卡住了。 | **流式传输 (SSE)**：后端收到 API 的流式响应后，立即推送到前端。或者**预加载**：鼠标还没滚到那里，先偷偷把下面的翻了。 |
| **样式错乱** | GitHub 的 CSS 很复杂，翻译文字挤在一起。 | **Shadow DOM**：虽然复杂点，但能完美隔离样式。或者给注入的 `div` 加上 `all: revert` 重置样式。 |
| **上下文丢失** | 翻译 "Issue" 时，有的翻成"问题"，有的翻成"发行"。 | **批量发送**：不要一句一句发。每次发送 20 句，让 LLM 看到上下文，它就知道这里是在谈论 GitHub Issues。 |

---

### 五、 速度预估

使用 DeepSeek V3 API 的情况下：

1.  **冷启动（无缓存）**：
    *   API 响应时间：约 **1.5s ~ 2.5s** (处理 20 个段落)。
    *   网络延迟：约 100ms。
    *   **用户体感**：点击按钮后，约 2 秒开始出现第一批翻译，随后像瀑布一样流下来。

2.  **热启动（有缓存）**：
    *   Golang 查 BadgerDB：< 5ms。
    *   **用户体感**：**毫秒级瞬间显示**。这才是你写这个工具的最大动力——你自己访问过的文档，第二次看就是双语版，完全免费且极速。

---

### 六、 执行 Roadmap (路线图)

建议分为三个阶段开发，不要一上来就追求完美。

#### 阶段 1：MVP (最小可行性产品) - 2天
*   **目标**：能跑通。
*   **后端**：Go 服务，写死 API Key，提供一个 POST 接口，不做缓存。
*   **前端**：Manifest V3 插件，只针对 `<p>` 标签进行翻译。
*   **测试**：在简单博客页面测试。

#### 阶段 2：GitHub 特化与缓存 - 3天
*   **后端**：接入 BadgerDB 本地文件缓存。加入针对 GitHub 的 Prompt 逻辑。
*   **前端**：优化选择器，排除 `<pre>` 和 `<code>`。美化注入的 CSS 样式（模仿 GitHub 风格）。

#### 阶段 3：并发与体验优化 - 2天
*   **后端**：实现并发池，防止 API 超限。
*   **前端**：添加“开关按钮”和“翻译进度条”。

---

### 总结

这个方案**放弃了庞大的本地模型维护成本**，利用 Go 的高性能并发处理能力去调度廉价且强大的云端 API。

你只需要支付极少的 API 费用，就能获得一个**完全由你掌控逻辑、隐私相对安全、且针对程序员场景深度优化**的沉浸式翻译工具。而且，因为缓存层的存在，用的越久，体验越快，费用越低。