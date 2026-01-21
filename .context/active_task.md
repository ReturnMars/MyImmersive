# 当前任务: 代码清理与模块化重构

## 🎯 目标 (Goal)

对 MVP 版本进行代码清理，建立清晰的项目模块结构，为后续功能开发做准备。

## 🚦 当前进度 (Status)

✅ **重构完成！** 所有步骤已执行并验证。

## 📝 执行计划 (Execution Plan)

### 后端清理 (Go Backend)

- [x] 步骤 1: 安全配置 - 移除硬编码 API Key，使用环境变量管理 ✅
- [x] 步骤 2: 模块化分层 - 拆分 main.go 为标准结构 ✅
- [x] 步骤 3: 错误处理规范化 - 定义统一的错误响应结构 ✅

### 插件清理 (Chrome Extension)

- [x] 步骤 4: 分离样式文件 ✅
- [~] 步骤 5: 模块化 JS (暂跳过，当前规模不需要)

### 项目规范

- [x] 步骤 6: 更新 `.gitignore` 补充常用忽略项 ✅
- [x] 步骤 7: 添加 `backend/.env.example` 配置模板 ✅
- [x] 步骤 8: 更新 `system_map.md` 反映新目录结构 ✅

## 🧠 状态与暂存区 (Scratchpad)

### ✅ 已完成里程碑

**阶段二：代码清理与模块化 (2026-01-21)**

| 变更项            | Before    | After                                           |
| ----------------- | --------- | ----------------------------------------------- |
| `main.go` 行数    | 145 行    | 31 行                                           |
| `content.js` 行数 | 201 行    | 144 行                                          |
| 后端目录结构      | 单文件    | config/ + internal/{handler,service,middleware} |
| API Key           | 硬编码 😱 | `.env` 环境变量 ✅                              |

### 📁 新增文件

- `backend/config/config.go` - 配置管理
- `backend/internal/handler/translate.go` - HTTP 处理器
- `backend/internal/service/translator.go` - 翻译服务
- `backend/internal/middleware/cors.go` - CORS 中间件
- `backend/.env` / `.env.example` - 配置文件
- `extension/styles/content.css` - 独立样式

### ⚠️ 下一步建议

1. **集成 BadgerDB 缓存** - 减少重复翻译请求
2. **批量翻译优化** - 当前是单段翻译，需支持批量
3. **UI/UX 优化** - 添加设置页面、快捷键
