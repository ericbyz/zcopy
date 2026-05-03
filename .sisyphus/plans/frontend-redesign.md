# ZCopy 前端设计更新 — Apple HIG 简约风格 + 明暗主题

## TL;DR

> **Quick Summary**: 对服务端前端 (server/front) 和客户端前端 (client/front) 进行全面的视觉重构和组件拆分，引入 Element Plus 组件库 + Lucide 图标库，实现 Apple HIG 简约风格，并添加跟随系统+手动切换的明暗主题系统。
> 
> **Deliverables**:
> - 统一设计系统（CSS 变量 + Element Plus 主题定制）
> - 明暗主题切换（跟随系统 + 手动切换 + localStorage 持久化）
> - 服务端前端：Element Plus 组件替换 + 视觉重设计
> - 客户端前端：Element Plus 组件替换 + 视觉重设计 + 组件拆分
> - Vitest 测试框架搭建 + 核心工具函数单元测试
> 
> **Estimated Effort**: Large
> **Parallel Execution**: YES - 5 waves
> **Critical Path**: Task 1 (依赖安装) → Task 3 (设计 Token) → Task 5-6 (组件替换) → Task 7-8 (组件拆分) → Task 9-10 (测试) → Final

---

## Context

### Original Request
用户认为服务端和客户端前端不够好看，希望简约设计并加入白天黑夜模式。

### Interview Summary
**Key Discussions**:
- 设计风格：选择 Apple HIG（大量留白、圆角、微妙阴影、半透明毛玻璃）
- 组件库：选择 Element Plus（企业级、中文生态好、内置 dark mode）
- 图标库：选择 Lucide Icons（简约线条风格）
- 统一设计系统：两个前端共享同一套 CSS 变量和设计规范
- 主题切换：跟随系统 prefers-color-scheme + 手动切换，localStorage 持久化
- 改造范围：视觉层重构 + 组件拆分（DashboardView.vue 663行拆分）
- 测试：搭建 Vitest + Vue Test Utils + 核心组件单元测试

**Research Findings**:
- macOS 客户端 (front-mac) 没有自己的 Vue 代码，直接消费 client/front/dist — 只需改两套代码
- 当前零 CSS 变量系统，颜色全部硬编码
- 两个前端风格一致但代码完全独立
- 当前 emoji 图标系统（📁📄）需要替换
- 服务端无路由（tab 切换），客户端有 Vue Router（hash 模式）

### Metis Review
**Identified Gaps** (all addressed):
- Element Plus tree-shaking：使用 unplugin-vue-components + unplugin-auto-import
- 共享设计系统位置：在各项目 style.css 中定义 CSS 变量，用注释标记同步版本
- 服务端前端不加 vue-router（保持 tab 切换）
- 客户端必须保持 `base: './'` 和 `createWebHashHistory()`
- Element Plus 需配置 zh-CN locale
- 需要防 FOUC 内联脚本
- 主题切换需配合 Element Plus 内置 dark mode（html.dark）
- 不包装 Element Plus 组件为自定义 Base 组件

---

## Work Objectives

### Core Objective
将两个前端从当前手写 CSS 暗色玻璃态风格，改造为 Apple HIG 简约风格，引入 Element Plus + Lucide Icons，实现完整的明暗主题切换系统，并拆分臃肿组件提升可维护性。

### Concrete Deliverables
- `server/front/` — 全新的 Apple HIG 风格 UI + 明暗主题
- `client/front/` — 全新的 Apple HIG 风格 UI + 明暗主题 + 拆分后的组件结构
- `client/front-mac/` — 自动受益于 client/front 改造（无需额外工作）
- 统一 CSS 变量设计系统（两个项目各自定义，注释标注同步）
- Vitest 测试框架 + 核心单元测试

### Definition of Done
- [ ] `cd server/front && npm run build` 成功
- [ ] `cd client/front && npm run build` 成功
- [ ] 服务端前端在浏览器中加载无 JS 控制台错误
- [ ] 客户端前端在 Electron 中加载无 JS 控制台错误
- [ ] 明暗主题切换正常工作（系统跟随 + 手动切换）
- [ ] Electron IPC（pickFolder、openPath）功能正常
- [ ] macOS 构建流程正常（client/front-mac npm run dist）

### Must Have
- Element Plus 组件替换所有手写 HTML 控件
- Lucide Icons 替换所有 emoji 图标
- CSS 变量系统支持 light/dark 双主题
- 跟随系统 + 手动切换的主题切换器
- localStorage 持久化主题偏好
- 防 FOUC 内联脚本
- Element Plus zh-CN locale 配置
- DashboardView.vue 拆分为 6+ 子组件
- 服务端 App.vue 拆分为子组件
- Vitest 测试框架 + 工具函数测试

### Must NOT Have (Guardrails)
- ❌ 不修改任何 Go 后端代码
- ❌ 不修改 Electron 主进程文件（electron/main.js, preload.js）
- ❌ 不给服务端前端添加 vue-router
- ❌ 不添加 Pinia/Vuex 状态管理
- ❌ 不包装 Element Plus 组件为自定义 Base 组件
- ❌ 不添加动画库（仅 CSS transition）
- ❌ 不转为 TypeScript
- ❌ 不添加超出已有的表单验证规则
- ❌ 不修改客户端 vite.config.js 中的 `base: './'`
- ❌ 不将客户端路由从 hash 模式改为 history 模式
- ❌ 不修改 localStorage key `zcopy_token`
- ❌ 不改变 API 请求/响应格式

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: NO（需从零搭建）
- **Automated tests**: YES（Vitest + Vue Test Utils）
- **Framework**: Vitest + @vue/test-utils
- **Test scope**: 纯工具函数 + 主题切换逻辑 + 组件 props/events

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Frontend/UI**: Use Playwright — 打开页面、切换主题、截图对比
- **Build**: Use Bash — npm run build、npm run dev、检查控制台
- **Tests**: Use Bash — npx vitest run

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (基础设施 — 全部串行前置):
├── Task 1: 安装依赖 + 配置 Vite 插件（两个前端） [quick]
├── Task 2: 配置 Element Plus zh-CN locale + 全局注册 [quick]
└── Task 3: 创建 CSS 设计 Token + 主题系统 [visual-engineering]

Wave 2 (服务端前端改造 — 可并行):
├── Task 4: 服务端前端 — 认证页面重构 [visual-engineering]
├── Task 5: 服务端前端 — 文件浏览器重构 [visual-engineering]
└── Task 6: 服务端前端 — LogViewer 组件重构 [visual-engineering]

Wave 3 (客户端前端改造 — 可并行):
├── Task 7: 客户端前端 — 认证页面 + Hero 区域重构 [visual-engineering]
├── Task 8: 客户端前端 — 任务表单 + 任务列表重构 [visual-engineering]
├── Task 9: 客户端前端 — 远程目录选择器重构 [visual-engineering]
└── Task 10: 客户端前端 — LogViewer 组件重构 [visual-engineering]

Wave 4 (组件拆分 + 测试):
├── Task 11: 客户端 — DashboardView.vue 拆分为子组件 [deep]
├── Task 12: 服务端 — App.vue 拆分为子组件 [deep]
└── Task 13: 搭建 Vitest + 核心单元测试 [unspecified-high]

Wave FINAL (验证 — 4 并行审查):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Build + code quality review (unspecified-high)
├── Task F3: Visual QA — both themes (unspecified-high + playwright)
└── Task F4: Scope fidelity check (deep)
-> Present results -> Get explicit user okay

Critical Path: T1 → T2 → T3 → T4-T6 → T7-T10 → T11-T13 → F1-F4 → user okay
Parallel Speedup: ~60% faster than sequential
Max Concurrent: 4 (Waves 3)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 1 | - | 2, 3 | 1 |
| 2 | 1 | 4-10 | 1 |
| 3 | 1 | 4-10 | 1 |
| 4 | 2, 3 | 12 | 2 |
| 5 | 2, 3 | 12 | 2 |
| 6 | 2, 3 | 12 | 2 |
| 7 | 2, 3 | 11 | 3 |
| 8 | 2, 3 | 11 | 3 |
| 9 | 2, 3 | 11 | 3 |
| 10 | 2, 3 | 11 | 3 |
| 11 | 7, 8, 9, 10 | F1-F4 | 4 |
| 12 | 4, 5, 6 | F1-F4 | 4 |
| 13 | 11, 12 | F1-F4 | 4 |

### Agent Dispatch Summary

- **Wave 1**: 3 tasks — T1 `quick`, T2 `quick`, T3 `visual-engineering`
- **Wave 2**: 3 tasks — T4 `visual-engineering`, T5 `visual-engineering`, T6 `visual-engineering`
- **Wave 3**: 4 tasks — T7-T10 all `visual-engineering`
- **Wave 4**: 3 tasks — T11 `deep`, T12 `deep`, T13 `unspecified-high`
- **FINAL**: 4 tasks — F1 `oracle`, F2 `unspecified-high`, F3 `unspecified-high`, F4 `deep`

---

## TODOs

- [x] 1. 安装依赖 + 配置 Vite 插件（两个前端）

  **What to do**:
  - 在 `server/front/` 和 `client/front/` 分别安装依赖：
    - `element-plus`、`@element-plus/icons-vue`
    - `lucide-vue-next`
    - `unplugin-vue-components`、`unplugin-auto-import`（dev）
  - 更新两个 `vite.config.js`：添加 `unplugin-vue-components` 和 `unplugin-auto-import` 插件，配置 Element Plus resolver 实现 tree-shaking
  - **关键约束**：客户端 `vite.config.js` 的 `base: './'` 必须保留不动
  - 在两个 `index.html` 中添加 `<meta name="color-scheme" content="light dark">`
  - 验证 `npm run build` 在两个项目中均成功

  **Must NOT do**:
  - 不修改 `base: './'`
  - 不添加 Pinia、Vuex、vue-router（服务端）
  - 不修改 Electron 相关文件

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 依赖安装和配置文件修改，逻辑清晰
  - **Skills**: [`frontend-design`]
    - `frontend-design`: 了解 Vite 插件配置和 Element Plus 集成最佳实践

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 1 (sequential start)
  - **Blocks**: Tasks 2, 3
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `server/front/vite.config.js` — 当前服务端 Vite 配置（无额外插件）
  - `client/front/vite.config.js` — 当前客户端 Vite 配置（`base: './'`，需保留）

  **API/Type References**:
  - `server/front/package.json` — 当前服务端依赖（axios, vue）
  - `client/front/package.json` — 当前客户端依赖（axios, vue, vue-router）

  **External References**:
  - Element Plus Vite 集成: `https://element-plus.org/en-US/guide/quickstart.html#on-demand-import`
  - Lucide Vue: `https://lucide.dev/guide/install/lucide-vue-next`

  **WHY Each Reference Matters**:
  - `vite.config.js`：需在现有配置基础上追加插件，不能覆盖已有设置
  - `package.json`：确认当前依赖列表，避免版本冲突

  **Acceptance Criteria**:
  - [ ] `cd server/front && npm run build` 成功（无错误）
  - [ ] `cd client/front && npm run build` 成功（无错误）
  - [ ] `client/front/vite.config.js` 中 `base: './'` 保持不变
  - [ ] 两个 `index.html` 含 `<meta name="color-scheme" content="light dark">`

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: 服务端构建成功
    Tool: Bash
    Preconditions: server/front/ 目录存在
    Steps:
      1. cd server/front && npm run build
      2. 检查 exit code 为 0
      3. 检查 dist/ 目录已生成
      4. 检查 dist/index.html 包含 "element-plus" 或 "element" 的引用
    Expected Result: 构建成功，dist/ 产物包含 Element Plus 代码
    Failure Indicators: exit code 非 0，或 dist/ 中无 Element Plus 引用
    Evidence: .sisyphus/evidence/task-1-server-build.txt

  Scenario: 客户端构建成功 + base 路径正确
    Tool: Bash
    Preconditions: client/front/ 目录存在
    Steps:
      1. cd client/front && npm run build
      2. 检查 exit code 为 0
      3. 检查 dist/index.html 中的资源路径使用相对路径（./）
      4. grep "base:" client/front/vite.config.js 确认 base: './' 仍存在
    Expected Result: 构建成功，资源路径为相对路径
    Failure Indicators: exit code 非 0，或资源路径使用绝对路径 /assets/
    Evidence: .sisyphus/evidence/task-1-client-build.txt
  ```

  **Commit**: YES
  - Message: `feat(frontend): install Element Plus, Lucide, and configure Vite plugins`
  - Files: `server/front/package.json`, `server/front/vite.config.js`, `server/front/index.html`, `client/front/package.json`, `client/front/vite.config.js`, `client/front/index.html`
  - Pre-commit: `cd server/front && npm run build && cd ../../client/front && npm run build`

- [x] 2. 配置 Element Plus zh-CN locale + 全局注册

  **What to do**:
  - 在 `server/front/src/main.js` 中：
    - 导入 Element Plus 中文 locale：`import zhCn from 'element-plus/es/locale/lang/zh-cn'`
    - 配置 `<el-config-provider :locale="zhCn">` 包裹整个 App
  - 在 `client/front/src/main.js` 中做同样的配置
  - 确认 Element Plus 的 `ElMessage`、`ElMessageBox`、`ElNotification`、`ElLoading` 指令可以全局使用
  - 在 App.vue 根组件中包裹 `<el-config-provider>`

  **Must NOT do**:
  - 不使用 `app.use(ElementPlus)` 全量注册（已通过 unplugin 按需加载）
  - 不创建全局 axios 实例模块

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 两个 main.js 文件的简单配置修改
  - **Skills**: [`frontend-design`]
    - `frontend-design`: 了解 Element Plus 全局配置模式

  **Parallelization**:
  - **Can Run In Parallel**: NO（依赖 Task 1 的依赖安装）
  - **Parallel Group**: Wave 1
  - **Blocks**: Tasks 4-10
  - **Blocked By**: Task 1

  **References**:

  **Pattern References**:
  - `server/front/src/main.js` — 当前服务端 Vue 启动（5行，简洁）
  - `client/front/src/main.js` — 当前客户端 Vue + Router 启动

  **External References**:
  - Element Plus i18n: `https://element-plus.org/en-US/guide/i18n.html`

  **WHY Each Reference Matters**:
  - `main.js`：需在现有 createApp 逻辑基础上添加 ConfigProvider，不能破坏已有功能

  **Acceptance Criteria**:
  - [ ] `server/front/src/main.js` 导入 zh-CN locale
  - [ ] `client/front/src/main.js` 导入 zh-CN locale
  - [ ] 两个 App.vue 根组件用 `<el-config-provider>` 包裹

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Element Plus locale 配置正确
    Tool: Bash
    Preconditions: npm run dev 已启动
    Steps:
      1. cd server/front && npm run dev &
      2. sleep 3
      3. curl -s http://localhost:5173 | grep -c "el-config-provider"
      4. kill dev server
    Expected Result: 页面 HTML 包含 el-config-provider
    Failure Indicators: 无 el-config-provider 或 curl 失败
    Evidence: .sisyphus/evidence/task-2-locale-config.txt

  Scenario: 构建不报错
    Tool: Bash
    Steps:
      1. cd server/front && npm run build
      2. cd ../../client/front && npm run build
    Expected Result: 两个项目构建成功
    Failure Indicators: 任何构建错误
    Evidence: .sisyphus/evidence/task-2-build-check.txt
  ```

  **Commit**: YES (groups with Task 1)
  - Message: `feat(frontend): configure Element Plus zh-CN locale`
  - Files: `server/front/src/main.js`, `client/front/src/main.js`, `server/front/src/App.vue`, `client/front/src/App.vue`

- [x] 3. 创建 CSS 设计 Token + 主题系统

  **What to do**:
  - 创建统一的设计 Token 系统（CSS custom properties），支持 light/dark 双主题
  - **Dark 模式色彩方案**（基于当前风格优化）：
    - 背景：`#0f172a`（保持深蓝基调）
    - 卡片：`rgba(15, 23, 42, 0.78)` + backdrop-filter: blur(18px)
    - 文本层级：primary `#f1f5f9` / secondary `#cbd5e1` / muted `#94a3b8`
    - 强调色：`#3b82f6`（蓝）
    - 成功色：`#22c55e`（绿）
    - 危险色：`#ef4444`（红）
    - 边框：`rgba(148, 163, 184, 0.15)`
  - **Light 模式色彩方案**（Apple HIG 灵感）：
    - 背景：`#f5f5f7`（Apple 标志性浅灰）
    - 卡片：`rgba(255, 255, 255, 0.72)` + backdrop-filter: saturate(180%) blur(20px)
    - 文本层级：primary `#1d1d1f` / secondary `#6e6e73` / muted `#86868b`
    - 强调色：`#0071e3`（Apple 蓝）
    - 成功色：`#34c759`（Apple 绿）
    - 危险色：`#ff3b30`（Apple 红）
    - 边框：`rgba(0, 0, 0, 0.08)`
  - 覆盖 Element Plus CSS 变量以匹配设计系统：
    - `--el-color-primary`、`--el-border-radius-base`（改为 12px，更 Apple HIG）
    - `--el-font-family`（使用系统字体栈）
    - 所有 Element Plus 颜色变量映射到设计 Token
  - 实现主题切换逻辑：
    - 检测 `prefers-color-scheme`
    - 读取 `localStorage('zcopy-theme')`（`'light'` / `'dark'` / `'auto'`）
    - 在 `<html>` 元素上添加/移除 `dark` class（Element Plus dark mode 要求）
  - 在两个 `index.html` 的 `<head>` 中添加防 FOUC 内联脚本：
    ```html
    <script>
      (function() {
        var t = localStorage.getItem('zcopy-theme');
        var d = t === 'auto' || !t ? window.matchMedia('(prefers-color-scheme: dark)').matches : t === 'dark';
        if (d) document.documentElement.classList.add('dark');
      })();
    </script>
    ```
  - 在两个 `style.css` 中写入完整 CSS 变量系统，头部加注释 `/* ZCopy Design Tokens v1 — sync with [other project] */`
  - 删除两个 `style.css` 中所有硬编码颜色值，替换为 CSS 变量引用
  - 重写 `body` 背景为渐变光斑效果（light/dark 各有不同色调）

  **Must NOT do**:
  - 不使用 JS 框架级别主题管理（保持纯 CSS + class 切换）
  - 不创建 `.ts` 类型定义文件
  - 不修改 API 调用逻辑

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: 设计系统创建需要视觉设计感知 + CSS 工程能力
  - **Skills**: [`frontend-design`, `ui-ux-pro-max`]
    - `frontend-design`: 设计 Token 系统和 Apple HIG 风格指南
    - `ui-ux-pro-max`: 颜色系统、dark mode 最佳实践

  **Parallelization**:
  - **Can Run In Parallel**: NO（依赖 Task 1 的 unplugin 配置）
  - **Parallel Group**: Wave 1
  - **Blocks**: Tasks 4-10
  - **Blocked By**: Task 1

  **References**:

  **Pattern References**:
  - `server/front/src/style.css` — 当前硬编码颜色值（264行），需全部替换为变量
  - `client/front/src/style.css` — 当前硬编码颜色值（408行），需全部替换为变量

  **API/Type References**:
  - `server/front/index.html` — 需在 `<head>` 中添加防 FOUC 脚本
  - `client/front/index.html` — 同上

  **External References**:
  - Element Plus Dark Mode: `https://element-plus.org/en-US/guide/dark-mode.html`
  - Element Plus 主题定制: `https://element-plus.org/en-US/guide/theming.html`
  - Apple HIG Color: `https://developer.apple.com/design/human-interface-guidelines/color`

  **WHY Each Reference Matters**:
  - `style.css`：是核心改造对象，所有硬编码颜色必须找到并替换
  - `index.html`：防 FOUC 脚本必须在 CSS 之前执行，所以放在 `<head>` 顶部
  - Element Plus dark mode 文档：说明 `html.dark` class 是其 dark mode 的触发方式

  **Acceptance Criteria**:
  - [ ] 两个 `style.css` 使用 CSS 变量引用，无硬编码颜色值
  - [ ] `html.dark` class 控制 dark mode
  - [ ] 防 FOUC 脚本存在于两个 `index.html`
  - [ ] `localStorage('zcopy-theme')` 可读取主题偏好
  - [ ] Element Plus CSS 变量被覆盖（`--el-color-primary` 等）

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Light 模式加载正确
    Tool: Bash (curl)
    Preconditions: server/front dev server 运行
    Steps:
      1. 确保无 localStorage zcopy-theme（清除）
      2. curl http://localhost:5173
      3. 检查返回 HTML 中 <html> 不含 "dark" class（模拟 light 系统）
    Expected Result: 默认无 dark class（light 模式）
    Failure Indicators: HTML 含 dark class（当无 localStorage 且系统为 light 时）
    Evidence: .sisyphus/evidence/task-3-light-mode.html

  Scenario: Dark 模式切换正确
    Tool: Bash (curl)
    Preconditions: server/front dev server 运行
    Steps:
      1. 用 curl 模拟带 zcopy-theme=dark 的请求
      2. 检查防 FOUC 脚本逻辑正确性（读脚本源码）
      3. grep "zcopy-theme" server/front/index.html 确认脚本存在
      4. grep "classList.add.*dark" server/front/index.html 确认 dark class 逻辑
    Expected Result: 防 FOUC 脚本存在且逻辑正确
    Failure Indicators: 脚本缺失或逻辑不完整
    Evidence: .sisyphus/evidence/task-3-dark-mode-check.txt

  Scenario: 无硬编码颜色残留
    Tool: Bash (grep)
    Steps:
      1. grep -E "#[0-9a-fA-F]{3,8}" server/front/src/style.css | grep -v "^/\*" | grep -v "var(--"
      2. 同样检查 client/front/src/style.css
      3. 排除 CSS 变量定义行（:root 和 html.dark 块内的颜色定义是允许的）
    Expected Result: 只有 CSS 变量定义块内有颜色字面量，其余全部引用变量
    Failure Indicators: 非变量定义的硬编码颜色值
    Evidence: .sisyphus/evidence/task-3-no-hardcoded-colors.txt
  ```

  **Commit**: YES (groups with Tasks 1-2)
  - Message: `feat(frontend): create CSS design token system with light/dark theme support`
  - Files: `server/front/src/style.css`, `client/front/src/style.css`, `server/front/index.html`, `client/front/index.html`

- [x] 4. 服务端前端 — 认证页面重构

  **What to do**:
  - 重写 `server/front/src/App.vue` 中认证部分（auth-layout / auth-card 区域）：
    - 登录/注册 Tab → `<el-tabs>` + `<el-tab-pane>`
    - 表单 inputs → `<el-form>` + `<el-form-item>` + `<el-input>`
    - 主按钮 → `<el-button type="primary">`
    - 消息提示 → `ElMessage.success()` / `ElMessage.error()` 替代 inline `.message`
    - 用户信息卡片 → `<el-card>` + `<el-descriptions>`
    - 退出按钮 → `<el-button>` (default type)
  - 添加主题切换按钮（放在 Hero 区域右上角）：
    - 使用 Lucide `Sun`、`Moon`、`Monitor` 图标
    - 点击切换：light → dark → auto → light
    - 当前状态显示对应图标
  - 将 Hero 标题区域使用系统字体栈（已在 Task 3 的 CSS 变量中定义）
  - 确保认证表单在 light/dark 两种模式下视觉正确

  **Must NOT do**:
  - 不添加 vue-router
  - 不添加表单验证规则（保持现有最小验证）
  - 不修改 API 调用逻辑

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: 视觉重构需要 Element Plus 组件知识和设计感知
  - **Skills**: [`frontend-design`, `ui-ux-pro-max`]
    - `frontend-design`: Element Plus 组件用法和 Apple HIG 样式
    - `ui-ux-pro-max`: dark mode 交互设计、表单布局

  **Parallelization**:
  - **Can Run In Parallel**: YES（与 Task 5, 6 并行）
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 12
  - **Blocked By**: Tasks 2, 3

  **References**:

  **Pattern References**:
  - `server/front/src/App.vue:267-307` — 当前认证区域模板（auth-layout + auth-card + tabs + form-grid）
  - `server/front/src/App.vue:270-280` — Hero 区域（需添加主题切换按钮）

  **API/Type References**:
  - `server/front/src/App.vue:64-95` — `submitAuth()` 函数（API 调用逻辑，不改）
  - `server/front/src/App.vue:51-53` — `setMessage()` 函数（改为 ElMessage 调用）

  **External References**:
  - Element Plus Tabs: `https://element-plus.org/en-US/component/tabs.html`
  - Element Plus Form: `https://element-plus.org/en-US/component/form.html`
  - Element Plus Message: `https://element-plus.org/en-US/component/message.html`
  - Lucide Vue: `https://lucide.dev/icons/?icon=sun,moon,monitor`

  **WHY Each Reference Matters**:
  - App.vue 认证区域：是本次改造的直接对象，需逐行替换为 Element Plus 组件
  - `setMessage()` 函数：当前用 inline `.message` div 显示消息，需改为 ElMessage 调用
  - Hero 区域：主题切换按钮的放置位置

  **Acceptance Criteria**:
  - [ ] 认证 Tab 使用 `<el-tabs>`
  - [ ] 表单使用 `<el-form>` + `<el-input>`
  - [ ] 消息使用 `ElMessage` 而非 inline div
  - [ ] 主题切换按钮存在（Sun/Moon/Monitor 图标）
  - [ ] Light 和 Dark 模式下认证页面视觉正确

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: 认证页面 Element Plus 组件替换
    Tool: Bash
    Steps:
      1. grep -c "el-tabs\|el-tab-pane" server/front/src/App.vue — 期望 >= 1
      2. grep -c "el-form" server/front/src/App.vue — 期望 >= 1
      3. grep -c "el-input" server/front/src/App.vue — 期望 >= 2
      4. grep -c "ElMessage" server/front/src/App.vue — 期望 >= 1
      5. grep -c "class=\"auth-card\"\|class=\"auth-layout\"" server/front/src/App.vue — 期望 0（旧类名已移除）
    Expected Result: Element Plus 组件已替换原有 HTML 元素
    Failure Indicators: 旧类名仍存在，或 Element Plus 组件计数不足
    Evidence: .sisyphus/evidence/task-4-auth-components.txt

  Scenario: 主题切换按钮存在
    Tool: Bash
    Steps:
      1. grep -E "Sun|Moon|Monitor|lucide-vue-next" server/front/src/App.vue
      2. grep "zcopy-theme" server/front/src/App.vue（确认主题切换逻辑）
    Expected Result: Lucide 图标导入且主题切换逻辑存在
    Failure Indicators: 无 Lucide 导入或无主题切换逻辑
    Evidence: .sisyphus/evidence/task-4-theme-toggle.txt
  ```

  **Commit**: YES
  - Message: `feat(server-front): redesign auth page with Element Plus + theme toggle`
  - Files: `server/front/src/App.vue`

- [x] 5. 服务端前端 — 文件浏览器重构

  **What to do**:
  - 重写 `server/front/src/App.vue` 中文件浏览器部分（workspace 区域）：
    - 文件列表 → `<el-table>` + `<el-table-column>`（名称、大小、时间、操作）
    - 目录图标 → `<Folder />` Lucide 图标，文件图标 → `<FileText />` Lucide 图标
    - 面包屑 → `<el-breadcrumb>` + `<el-breadcrumb-item>`
    - 工具栏按钮 → `<el-button>` 系列
    - 文件上传 → `<el-upload>`（保持原有 axios 上传逻辑）
    - 新建文件夹输入框 → `<el-input>` + `<el-button>`
    - 删除确认 → `ElMessageBox.confirm()`
    - 刷新按钮 → `<el-button>` + `<RefreshCw />` Lucide 图标
    - 加载状态 → `v-loading` 指令
    - 空状态 → `<el-empty>`
  - 文件名点击行为保留：目录 → 导航进入，文件 → 下载
  - 确保 `<el-table>` 在 light/dark 模式下正确渲染

  **Must NOT do**:
  - 不修改文件下载逻辑（保持 blob 下载）
  - 不添加分页（当前未分页）
  - 不修改 API 请求/响应格式

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: 数据表格和交互组件的视觉重构
  - **Skills**: [`frontend-design`, `ui-ux-pro-max`]
    - `frontend-design`: Element Plus Table、Upload、Breadcrumb 组件
    - `ui-ux-pro-max`: 表格布局、响应式设计

  **Parallelization**:
  - **Can Run In Parallel**: YES（与 Task 4, 6 并行）
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 12
  - **Blocked By**: Tasks 2, 3

  **References**:

  **Pattern References**:
  - `server/front/src/App.vue:309-364` — 当前文件浏览器 workspace 区域
  - `server/front/src/App.vue:316-331` — toolbar（breadcrumb + actions）
  - `server/front/src/App.vue:334-338` — upload-bar
  - `server/front/src/App.vue:340-363` — file-list（header + rows）

  **API/Type References**:
  - `server/front/src/App.vue:110-127` — `fetchFiles()` 函数
  - `server/front/src/App.vue:173-198` — `submitUpload()` 函数
  - `server/front/src/App.vue:200-225` — `downloadItem()` 函数
  - `server/front/src/App.vue:235-248` — `removeItem()` 函数

  **External References**:
  - Element Plus Table: `https://element-plus.org/en-US/component/table.html`
  - Element Plus Upload: `https://element-plus.org/en-US/component/upload.html`
  - Element Plus Breadcrumb: `https://element-plus.org/en-US/component/breadcrumb.html`
  - Element Plus MessageBox: `https://element-plus.org/en-US/component/message-box.html`

  **WHY Each Reference Matters**:
  - 文件浏览器是服务端前端的核心功能，需确保交互逻辑完全保留
  - `downloadItem()` 使用 blob 下载，不能被 Upload 组件替换
  - `removeItem()` 需添加确认对话框（ElMessageBox.confirm）

  **Acceptance Criteria**:
  - [ ] 文件列表使用 `<el-table>`
  - [ ] emoji 图标已替换为 Lucide 图标
  - [ ] 面包屑使用 `<el-breadcrumb>`
  - [ ] 删除操作有确认对话框
  - [ ] 加载状态使用 `v-loading`
  - [ ] 空状态使用 `<el-empty>`

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: 文件浏览器 Element Plus 组件替换
    Tool: Bash
    Steps:
      1. grep -c "el-table" server/front/src/App.vue — 期望 >= 1
      2. grep -c "el-breadcrumb" server/front/src/App.vue — 期望 >= 1
      3. grep -c "ElMessageBox" server/front/src/App.vue — 期望 >= 1
      4. grep -E "📁|📄" server/front/src/App.vue — 期望 0（emoji 已替换）
      5. grep -c "lucide-vue-next" server/front/src/App.vue — 期望 >= 1
    Expected Result: Element Plus 组件替换完成，无 emoji 残留
    Failure Indicators: 旧 HTML 结构残留或 emoji 仍在
    Evidence: .sisyphus/evidence/task-5-file-browser.txt

  Scenario: 构建成功无报错
    Tool: Bash
    Steps:
      1. cd server/front && npm run build
    Expected Result: exit code 0，无编译错误
    Failure Indicators: 任何构建错误
    Evidence: .sisyphus/evidence/task-5-build.txt
  ```

  **Commit**: YES
  - Message: `feat(server-front): redesign file browser with Element Plus components`
  - Files: `server/front/src/App.vue`

- [x] 6. 服务端前端 — LogViewer 组件重构

  **What to do**:
  - 重写 `server/front/src/components/LogViewer.vue`：
    - 日志级别筛选 → `<el-select>` + `<el-option>`
    - 关键字搜索 → `<el-input>` + `<Search />` Lucide 图标
    - 日期范围 → `<el-date-picker type="daterange">`
    - 日志列表 → `<el-table>` + `<el-table-column>`（时间、级别、消息）
    - 级别标签 → `<el-tag>`（info=blue, warn=yellow, error=red, debug=gray）
    - 分页 → `<el-pagination>`
    - 导出按钮 → `<el-button>` + `<Download />` Lucide 图标
    - 加载状态 → `v-loading`
    - 空状态 → `<el-empty>`
  - 保留所有现有过滤和分页逻辑（JS 逻辑不变）
  - 确保 scoped styles 改为使用 CSS 变量而非硬编码颜色

  **Must NOT do**:
  - 不修改 API 调用逻辑
  - 不添加新的过滤功能
  - 不将 scoped styles 中硬编码颜色保留（全部使用 CSS 变量）

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: 日志查看器的组件和视觉重构
  - **Skills**: [`frontend-design`]
    - `frontend-design`: Element Plus Table、Select、DatePicker、Pagination 组件

  **Parallelization**:
  - **Can Run In Parallel**: YES（与 Task 4, 5 并行）
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 12
  - **Blocked By**: Tasks 2, 3

  **References**:

  **Pattern References**:
  - `server/front/src/components/LogViewer.vue` — 当前完整组件（331行），需全面改造

  **External References**:
  - Element Plus Select: `https://element-plus.org/en-US/component/select.html`
  - Element Plus DatePicker: `https://element-plus.org/en-US/component/date-picker.html`
  - Element Plus Pagination: `https://element-plus.org/en-US/component/pagination.html`
  - Element Plus Tag: `https://element-plus.org/en-US/component/tag.html`

  **WHY Each Reference Matters**:
  - LogViewer 是服务端前端唯一的独立组件，改造模式将作为客户端 LogViewer 的参考
  - 当前使用手写 select/input/pagination，需逐一替换

  **Acceptance Criteria**:
  - [ ] 日志列表使用 `<el-table>`
  - [ ] 筛选使用 `<el-select>` + `<el-date-picker>`
  - [ ] 级别使用 `<el-tag>`
  - [ ] 分页使用 `<el-pagination>`
  - [ ] scoped styles 中无硬编码颜色值

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: LogViewer Element Plus 组件替换
    Tool: Bash
    Steps:
      1. grep -c "el-table" server/front/src/components/LogViewer.vue — 期望 >= 1
      2. grep -c "el-select" server/front/src/components/LogViewer.vue — 期望 >= 1
      3. grep -c "el-tag" server/front/src/components/LogViewer.vue — 期望 >= 1
      4. grep -c "el-pagination" server/front/src/components/LogViewer.vue — 期望 >= 1
    Expected Result: Element Plus 组件替换完成
    Failure Indicators: 计数为 0 的组件
    Evidence: .sisyphus/evidence/task-6-logviewer.txt

  Scenario: 无硬编码颜色残留
    Tool: Bash
    Steps:
      1. grep -E "#[0-9a-fA-F]{3,8}" server/front/src/components/LogViewer.vue | grep -v "^/\*" | grep -v "var(--" | grep -v "<!--"
    Expected Result: 无结果（颜色全部通过 CSS 变量）
    Failure Indicators: 存在硬编码颜色值
    Evidence: .sisyphus/evidence/task-6-no-hardcoded-colors.txt
  ```

  **Commit**: YES
  - Message: `feat(server-front): redesign LogViewer with Element Plus components`
  - Files: `server/front/src/components/LogViewer.vue`

---

- [x] 7. 客户端前端 — 认证页面 + Hero 区域重构

  **What to do**:
  - 重写 `client/front/src/views/DashboardView.vue` 中认证部分：
    - 登录/注册 Tab → `<el-tabs>` + `<el-tab-pane>`
    - 表单 → `<el-form>` + `<el-input>`
    - 按钮 → `<el-button>`
    - 消息 → `ElMessage` 替代 inline `.message`
  - 重写 Hero 区域：
    - 标题 + 描述保持简洁 Apple HIG 风格
    - 用户信息 → `<el-card>` 内含 `<el-descriptions>`
    - 退出按钮 → `<el-button>`
    - 添加主题切换按钮（同 Task 4 的模式）
  - 重写 `client/front/src/App.vue` 导航栏：
    - 路由链接 → `<el-menu>` mode="horizontal" 或自定义 `<el-button>` 组
    - 导航卡片 → `<el-card>` 或去掉卡片直接用 flex 布局
  - 保留 `window.desktopApi` 调用不变

  **Must NOT do**:
  - 不修改 `window.desktopApi` 相关代码
  - 不添加 vue-router 新路由
  - 不添加表单验证规则

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: 认证和导航区域的视觉重构
  - **Skills**: [`frontend-design`]
    - `frontend-design`: Element Plus 组件和 Apple HIG 布局

  **Parallelization**:
  - **Can Run In Parallel**: YES（与 Task 8, 9, 10 并行）
  - **Parallel Group**: Wave 3
  - **Blocks**: Task 11
  - **Blocked By**: Tasks 2, 3

  **References**:

  **Pattern References**:
  - `client/front/src/views/DashboardView.vue:458-496` — 当前认证区域模板
  - `client/front/src/views/DashboardView.vue:461-471` — Hero 区域
  - `client/front/src/App.vue:14-32` — 当前导航栏模板

  **API/Type References**:
  - `client/front/src/views/DashboardView.vue:126-159` — `submitAuth()` 函数
  - `client/front/src/views/DashboardView.vue:406-424` — `logout()` 函数

  **WHY Each Reference Matters**:
  - 认证逻辑完全保留，只替换模板和样式
  - Hero 区域需要添加主题切换按钮

  **Acceptance Criteria**:
  - [ ] 认证 Tab 使用 `<el-tabs>`
  - [ ] 表单使用 `<el-form>`
  - [ ] 消息使用 `ElMessage`
  - [ ] 导航栏使用 Element Plus 组件
  - [ ] 主题切换按钮存在

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: 客户端认证组件替换
    Tool: Bash
    Steps:
      1. grep -c "el-tabs" client/front/src/views/DashboardView.vue — 期望 >= 1
      2. grep -c "el-form" client/front/src/views/DashboardView.vue — 期望 >= 1
      3. grep -c "ElMessage" client/front/src/views/DashboardView.vue — 期望 >= 1
      4. grep -c "el-menu\|el-button" client/front/src/App.vue — 期望 >= 1
    Expected Result: Element Plus 组件已替换
    Failure Indicators: 组件计数不足
    Evidence: .sisyphus/evidence/task-7-client-auth.txt

  Scenario: desktopApi 调用未受影响
    Tool: Bash
    Steps:
      1. grep -c "window.desktopApi" client/front/src/views/DashboardView.vue — 期望 >= 1
      2. grep "pickFolder\|openPath" client/front/src/views/DashboardView.vue
    Expected Result: desktopApi 调用代码完整保留
    Failure Indicators: 计数为 0
    Evidence: .sisyphus/evidence/task-7-desktop-api-preserved.txt
  ```

  **Commit**: YES
  - Message: `feat(client-front): redesign auth + hero + navigation with Element Plus`
  - Files: `client/front/src/views/DashboardView.vue`, `client/front/src/App.vue`

- [x] 8. 客户端前端 — 任务表单 + 任务列表重构

  **What to do**:
  - 重写 `client/front/src/views/DashboardView.vue` 中任务相关部分：
    - 任务表单卡片 → `<el-card>` + `<el-form>`
    - 任务名称输入 → `<el-input>`
    - 本地目录输入 → `<el-input>` + `<el-button>`（保留 pickFolder 桌面选择器）
    - 远程目录输入 → `<el-input>` + `<el-button>`（保留远程选择器触发）
    - 复选框 → `<el-checkbox>`
    - 任务列表 → `<el-card>` 内含循环的 `<el-card>` 或手写卡片（使用 CSS 变量）
    - 状态标签 → `<el-tag>`（idle=info, syncing=success, failed=danger）
    - 进度条 → `<el-progress :percentage="syncProgress(task)">`
    - 同步报告 → 使用 `<el-descriptions>` 展示详情
    - 操作按钮 → `<el-button>` 组
    - 错误信息 → `<el-alert type="error">`
    - 平台能力 → `<el-descriptions>`
    - 空状态 → `<el-empty>`

  **Must NOT do**:
  - 不修改 API 调用逻辑
  - 不改变 2.5s 轮询定时器位置（仍在 DashboardView 级别）
  - 不添加新功能

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: 任务管理界面的视觉重构，组件较多
  - **Skills**: [`frontend-design`, `ui-ux-pro-max`]
    - `frontend-design`: Element Plus Card、Form、Progress、Tag 组件
    - `ui-ux-pro-max`: 表单布局、信息密度优化

  **Parallelization**:
  - **Can Run In Parallel**: YES（与 Task 7, 9, 10 并行）
  - **Parallel Group**: Wave 3
  - **Blocks**: Task 11
  - **Blocked By**: Tasks 2, 3

  **References**:

  **Pattern References**:
  - `client/front/src/views/DashboardView.vue:498-534` — 任务表单区域
  - `client/front/src/views/DashboardView.vue:536-608` — 任务列表区域
  - `client/front/src/views/DashboardView.vue:544-577` — 单个 task-row（状态、进度、错误）
  - `client/front/src/views/DashboardView.vue:580-607` — 任务操作 + on-demand panel
  - `client/front/src/views/DashboardView.vue:610-616` — 平台能力信息卡

  **API/Type References**:
  - `client/front/src/views/DashboardView.vue:272-295` — `saveTask()` 函数
  - `client/front/src/views/DashboardView.vue:340-354` — `deleteTask()` 函数
  - `client/front/src/views/DashboardView.vue:356-367` — `syncTask()` 函数
  - `client/front/src/views/DashboardView.vue:369-385` — `toggleAutoBackup()` 函数

  **External References**:
  - Element Plus Progress: `https://element-plus.org/en-US/component/progress.html`
  - Element Plus Alert: `https://element-plus.org/en-US/component/alert.html`
  - Element Plus Descriptions: `https://element-plus.org/en-US/component/descriptions.html`

  **WHY Each Reference Matters**:
  - 任务列表是客户端最核心的 UI，需确保所有交互保留
  - `syncProgress()` 函数计算的进度值需传给 `<el-progress>`
  - `taskStatusClass()` 的逻辑需映射到 `<el-tag>` 的 type prop

  **Acceptance Criteria**:
  - [ ] 任务表单使用 `<el-form>` + `<el-input>` + `<el-checkbox>`
  - [ ] 状态标签使用 `<el-tag>`
  - [ ] 进度条使用 `<el-progress>`
  - [ ] 错误信息使用 `<el-alert>`
  - [ ] 空状态使用 `<el-empty>`
  - [ ] emoji 图标已全部替换

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: 任务区域 Element Plus 组件替换
    Tool: Bash
    Steps:
      1. grep -c "el-form" client/front/src/views/DashboardView.vue — 期望 >= 1
      2. grep -c "el-tag" client/front/src/views/DashboardView.vue — 期望 >= 1
      3. grep -c "el-progress" client/front/src/views/DashboardView.vue — 期望 >= 1
      4. grep -c "el-empty" client/front/src/views/DashboardView.vue — 期望 >= 1
      5. grep -E "📁|📄" client/front/src/views/DashboardView.vue — 期望 0
    Expected Result: 所有 Element Plus 组件就位，无 emoji 残留
    Failure Indicators: 组件缺失或 emoji 残留
    Evidence: .sisyphus/evidence/task-8-task-components.txt

  Scenario: 构建成功
    Tool: Bash
    Steps:
      1. cd client/front && npm run build
    Expected Result: exit code 0
    Failure Indicators: 任何构建错误
    Evidence: .sisyphus/evidence/task-8-build.txt
  ```

  **Commit**: YES
  - Message: `feat(client-front): redesign task form + task list with Element Plus`
  - Files: `client/front/src/views/DashboardView.vue`

- [x] 9. 客户端前端 — 远程目录选择器重构

  **What to do**:
  - 重写远程目录选择器（picker-mask / picker 区域）：
    - 遮罩 + 模态 → `<el-dialog>` （width="720px"）
    - 面包屑 → `<el-breadcrumb>` + `<el-breadcrumb-item>`
    - 返回上级/选择当前目录按钮 → `<el-button>`
    - 文件夹列表 → `<el-table>` 或保持 button 列表但使用 Element Plus 样式
    - 每行文件夹 → `<Folder />` Lucide 图标替代 emoji 📁
    - 加载状态 → `v-loading`
    - 错误信息 → `<el-alert type="error">`
    - 空状态 → `<el-empty>`
  - 保留所有导航逻辑（navigateRemote、navigateRemoteParent、chooseRemotePath）

  **Must NOT do**:
  - 不修改远程目录 API 调用
  - 不添加新的文件操作功能

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: 模态框和目录浏览器的视觉重构
  - **Skills**: [`frontend-design`]
    - `frontend-design`: Element Plus Dialog、Breadcrumb、Table 组件

  **Parallelization**:
  - **Can Run In Parallel**: YES（与 Task 7, 8, 10 并行）
  - **Parallel Group**: Wave 3
  - **Blocks**: Task 11
  - **Blocked By**: Tasks 2, 3

  **References**:

  **Pattern References**:
  - `client/front/src/views/DashboardView.vue:620-660` — 当前远程目录选择器模态框

  **API/Type References**:
  - `client/front/src/views/DashboardView.vue:297-313` — `loadRemoteFolders()` 函数
  - `client/front/src/views/DashboardView.vue:315-327` — `openRemotePicker()` / `closeRemotePicker()` / `chooseRemotePath()` 函数
  - `client/front/src/views/DashboardView.vue:329-338` — `navigateRemote()` / `navigateRemoteParent()` 函数

  **External References**:
  - Element Plus Dialog: `https://element-plus.org/en-US/component/dialog.html`

  **WHY Each Reference Matters**:
  - 当前使用手写 `.picker-mask` + `.picker` 模态框，需替换为 `<el-dialog>` 的 v-model 控制模式
  - `remotePickerOpen` ref 直接映射到 `<el-dialog>` 的 `model-value`

  **Acceptance Criteria**:
  - [ ] 模态框使用 `<el-dialog>`
  - [ ] 面包屑使用 `<el-breadcrumb>`
  - [ ] 文件夹使用 Lucide Folder 图标
  - [ ] 旧 picker-mask/picker 样式类已移除

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: 远程选择器使用 ElDialog
    Tool: Bash
    Steps:
      1. grep -c "el-dialog" client/front/src/views/DashboardView.vue — 期望 >= 1
      2. grep -c "el-breadcrumb" client/front/src/views/DashboardView.vue — 期望 >= 1
      3. grep -c "class=\"picker-mask\"\|class=\"picker\"" client/front/src/views/DashboardView.vue — 期望 0
      4. grep -c "Folder" client/front/src/views/DashboardView.vue — 期望 >= 1（Lucide 图标）
    Expected Result: ElDialog 替换完成，旧类名已移除
    Failure Indicators: 旧 picker 类名仍存在
    Evidence: .sisyphus/evidence/task-9-remote-picker.txt
  ```

  **Commit**: YES
  - Message: `feat(client-front): redesign remote folder picker with ElDialog`
  - Files: `client/front/src/views/DashboardView.vue`

- [x] 10. 客户端前端 — LogViewer 组件重构

  **What to do**:
  - 重写 `client/front/src/views/LogViewer.vue`，遵循与 Task 6（服务端 LogViewer）相同的设计模式：
    - 日志列表 → `<el-table>`
    - 筛选 → `<el-select>` + `<el-input>` + `<el-date-picker>`
    - 级别标签 → `<el-tag>`
    - 分页 → `<el-pagination>`
    - 导出按钮 → `<el-button>` + `<Download />` Lucide 图标
    - 加载状态 → `v-loading`
    - 空状态 → `<el-empty>`
    - Hero 标题区域 → 简洁 `<el-card>` 或去掉卡片直接用标题
  - **特别注意**：当前客户端 LogViewer 使用 `fetch()` 而非 `axios`，本次统一改为 `axios`（与服务端一致）
  - 确保无硬编码颜色值

  **Must NOT do**:
  - 不添加新的过滤功能
  - 不修改后端 API

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: LogViewer 的视觉重构
  - **Skills**: [`frontend-design`]
    - `frontend-design`: Element Plus 组件

  **Parallelization**:
  - **Can Run In Parallel**: YES（与 Task 7, 8, 9 并行）
  - **Parallel Group**: Wave 3
  - **Blocks**: Task 11
  - **Blocked By**: Tasks 2, 3

  **References**:

  **Pattern References**:
  - `client/front/src/views/LogViewer.vue` — 当前完整组件（305行）
  - Task 6 改造后的 `server/front/src/components/LogViewer.vue` — 可参考的设计模式

  **External References**:
  - Element Plus Table/Select/DatePicker/Pagination/Tag（同 Task 6）

  **WHY Each Reference Matters**:
  - 客户端 LogViewer 与服务端 LogViewer 功能相似，应保持一致的设计模式
  - `fetch()` → `axios` 统一是 Metis 建议的改进

  **Acceptance Criteria**:
  - [ ] 日志列表使用 `<el-table>`
  - [ ] 筛选使用 `<el-select>` + `<el-date-picker>`
  - [ ] 级别使用 `<el-tag>`
  - [ ] 分页使用 `<el-pagination>`
  - [ ] `fetch()` 已替换为 `axios`
  - [ ] 无硬编码颜色值

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: 客户端 LogViewer Element Plus 组件替换
    Tool: Bash
    Steps:
      1. grep -c "el-table" client/front/src/views/LogViewer.vue — 期望 >= 1
      2. grep -c "el-pagination" client/front/src/views/LogViewer.vue — 期望 >= 1
      3. grep -c "fetch(" client/front/src/views/LogViewer.vue — 期望 0（已替换为 axios）
      4. grep -c "axios" client/front/src/views/LogViewer.vue — 期望 >= 1
    Expected Result: Element Plus 组件就位，fetch 已替换
    Failure Indicators: fetch 调用仍存在
    Evidence: .sisyphus/evidence/task-10-client-logviewer.txt
  ```

  **Commit**: YES
  - Message: `feat(client-front): redesign LogViewer with Element Plus + unify to axios`
  - Files: `client/front/src/views/LogViewer.vue`

---

- [x] 11. 客户端 — DashboardView.vue 拆分为子组件

  **What to do**:
  - 将 `client/front/src/views/DashboardView.vue`（当前 ~600+ 行）拆分为以下子组件：

  ```
  client/front/src/views/
  ├── DashboardView.vue          (~80 行 — 状态管理 + 子组件编排)
  ├── components/
  │   ├── AuthCard.vue           (~70 行 — 登录/注册表单)
  │   ├── TaskForm.vue           (~100 行 — 新建/编辑任务表单)
  │   ├── TaskList.vue           (~60 行 — 任务列表容器 + 空状态)
  │   ├── TaskCard.vue           (~130 行 — 单个任务：状态/进度/操作/同步报告/按需同步)
  │   ├── RemoteFolderPicker.vue (~80 行 — 远程目录选择模态框)
  │   └── PlatformInfo.vue       (~30 行 — 平台能力信息卡)
  ```

  - **状态管理原则**：
    - `DashboardView.vue` 保持所有 API 状态（tasks, currentUser, token, loading, capabilities, onDemandStatuses）
    - `DashboardView.vue` 保持 2.5s 轮询定时器（`startRefreshTimer` / `stopRefreshTimer`）
    - 子组件通过 `props` 接收数据，通过 `emit` 触发操作
    - 例如：`TaskCard` emit `edit`, `sync`, `toggle-auto`, `delete` 事件
    - `AuthCard` emit `submit` 事件，parent 处理 API 调用
  - **关键约束**：拆分后功能必须与拆分前完全一致，不增不减
  - 创建 `client/front/src/views/components/` 目录存放子组件

  **Must NOT do**:
  - 不将 2.5s 轮询移到子组件中
  - 不添加 Pinia/Vuex
  - 不改变任何 API 调用逻辑
  - 不将 `window.desktopApi` 调用移到子组件中（保持在 DashboardView 或 TaskForm 中通过 emit 桥接）

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: 组件拆分需要理解完整的状态流和事件流，需要深入思考
  - **Skills**: [`frontend-design`]
    - `frontend-design`: Vue 组件拆分最佳实践

  **Parallelization**:
  - **Can Run In Parallel**: NO（依赖 Wave 3 完成后的客户端前端代码）
  - **Parallel Group**: Wave 4
  - **Blocks**: Task 13, F1-F4
  - **Blocked By**: Tasks 7, 8, 9, 10

  **References**:

  **Pattern References**:
  - `client/front/src/views/DashboardView.vue` — 被拆分的源文件
  - `client/front/src/App.vue` — 参考 App 的 shell 模式（简洁编排器）

  **API/Type References**:
  - `client/front/src/views/DashboardView.vue:1-456` — `<script setup>` 中所有状态和函数（需分析哪些留在 parent，哪些移到子组件）
  - `client/front/src/views/DashboardView.vue:445-455` — `onMounted` / `onUnmounted`（定时器生命周期，留在 parent）

  **External References**:
  - Vue 3 组件通信: `https://vuejs.org/guide/components/events.html`

  **WHY Each Reference Matters**:
  - 整个 DashboardView 是拆分对象，需逐段分析哪些逻辑属于哪个子组件
  - 定时器和 mounted/unmounted 钩子必须在 parent 层级

  **Acceptance Criteria**:
  - [ ] `DashboardView.vue` 行数 < 120 行
  - [ ] 存在 6 个子组件文件（AuthCard, TaskForm, TaskList, TaskCard, RemoteFolderPicker, PlatformInfo）
  - [ ] 2.5s 轮询仍在 DashboardView 级别
  - [ ] `npm run build` 成功
  - [ ] 所有原有功能正常（登录、任务 CRUD、同步、远程选择器、on-demand）

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: 组件拆分结构正确
    Tool: Bash
    Steps:
      1. wc -l client/front/src/views/DashboardView.vue — 期望 < 120
      2. ls client/front/src/views/components/ — 期望 6 个 .vue 文件
      3. grep -c "setInterval\|refreshTimer" client/front/src/views/DashboardView.vue — 期望 >= 1（轮询在 parent）
      4. grep -c "Pinia\|useStore\|createStore" client/front/src/views/DashboardView.vue — 期望 0
    Expected Result: 拆分结构正确，轮询在 parent，无状态管理库
    Failure Indicators: DashboardView 行数过多，子组件缺失，轮询被移走
    Evidence: .sisyphus/evidence/task-11-component-split.txt

  Scenario: 构建成功
    Tool: Bash
    Steps:
      1. cd client/front && npm run build
    Expected Result: exit code 0，无编译错误
    Failure Indicators: 任何 import 错误或组件引用错误
    Evidence: .sisyphus/evidence/task-11-build.txt
  ```

  **Commit**: YES
  - Message: `refactor(client-front): split DashboardView into focused sub-components`
  - Files: `client/front/src/views/DashboardView.vue`, `client/front/src/views/components/*.vue`

- [x] 12. 服务端 — App.vue 拆分为子组件

  **What to do**:
  - 将 `server/front/src/App.vue` 拆分为以下子组件：

  ```
  server/front/src/
  ├── App.vue                    (~60 行 — 状态管理 + 条件渲染 + 主题切换)
  ├── components/
  │   ├── AuthCard.vue           (~70 行 — 登录/注册表单)
  │   ├── FileBrowser.vue        (~120 行 — 面包屑 + 上传 + 文件表格)
  │   ├── ThemeToggle.vue        (~30 行 — 主题切换按钮，可复用)
  │   └── LogViewer.vue          (已存在，保持不变)
  ```

  - **状态管理原则**：
    - `App.vue` 保持全局状态（token, currentUser, loading, message, activeTab）
    - 子组件通过 `props` + `emit` 通信
    - `FileBrowser` emit `navigate`, `upload`, `create-folder`, `delete` 事件
    - `AuthCard` emit `submit` 事件
  - `ThemeToggle.vue` 抽取为独立组件（可被两个前端复用的模式）
  - 创建 `server/front/src/components/` 目录（已存在，含 LogViewer）

  **Must NOT do**:
  - 不添加 vue-router
  - 不添加 Pinia/Vuex
  - 不改变 API 调用逻辑

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: 组件拆分需要状态流分析
  - **Skills**: [`frontend-design`]
    - `frontend-design`: Vue 组件拆分

  **Parallelization**:
  - **Can Run In Parallel**: YES（与 Task 11 并行）
  - **Parallel Group**: Wave 4
  - **Blocks**: Task 13, F1-F4
  - **Blocked By**: Tasks 4, 5, 6

  **References**:

  **Pattern References**:
  - `server/front/src/App.vue` — 被拆分的源文件
  - `server/front/src/components/LogViewer.vue` — 已有子组件（保持不变）

  **API/Type References**:
  - `server/front/src/App.vue:1-265` — `<script setup>` 中所有状态和函数

  **WHY Each Reference Matters**:
  - App.vue 的认证和文件浏览器是两个独立关注点，适合拆分
  - LogViewer 已经是子组件，不需要改动

  **Acceptance Criteria**:
  - [ ] `App.vue` 行数 < 80 行
  - [ ] 存在 AuthCard.vue, FileBrowser.vue, ThemeToggle.vue
  - [ ] LogViewer.vue 未被修改（或仅有 import 路径调整）
  - [ ] `npm run build` 成功

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: 服务端组件拆分结构
    Tool: Bash
    Steps:
      1. wc -l server/front/src/App.vue — 期望 < 80
      2. ls server/front/src/components/ — 期望 AuthCard.vue, FileBrowser.vue, ThemeToggle.vue, LogViewer.vue
      3. cd server/front && npm run build
    Expected Result: 拆分正确且构建成功
    Failure Indicators: App.vue 行数过多，构建失败
    Evidence: .sisyphus/evidence/task-12-server-split.txt
  ```

  **Commit**: YES
  - Message: `refactor(server-front): split App.vue into focused sub-components`
  - Files: `server/front/src/App.vue`, `server/front/src/components/AuthCard.vue`, `server/front/src/components/FileBrowser.vue`, `server/front/src/components/ThemeToggle.vue`

- [x] 13. 搭建 Vitest + 核心单元测试

  **What to do**:
  - 在 `server/front/` 和 `client/front/` 分别搭建 Vitest 测试框架：
    - 安装 dev 依赖：`vitest`、`@vue/test-utils`、`@vitejs/plugin-vue`（已有）、`jsdom`
    - 创建 `vitest.config.js`（复用 vite.config.js 的 resolve 和 plugins）
    - 在 `package.json` 添加 `"test": "vitest run"` 和 `"test:watch": "vitest"` scripts
  - 编写纯工具函数测试（两个项目各自需要的）：
    - **服务端**：
      - `breadcrumbList` computed 逻辑（路径拆分为面包屑数组）
      - `parseDownloadFilename()` content-disposition 解析
    - **客户端**：
      - `formatBytes()` 数值格式化
      - `formatSpeed()` 速度格式化
      - `taskStatusText()` 状态映射
      - `syncProgress()` 进度百分比计算
  - 编写组件测试（至少 1 个）：
    - `ThemeToggle.vue` — 测试切换逻辑（dark → light → auto → dark）
  - 确保 `npx vitest run` 在两个项目中均通过

  **Must NOT do**:
  - 不写 E2E 测试（无 Playwright/Cypress）
  - 不追求高覆盖率（仅核心逻辑）
  - 不为 Element Plus 组件写测试（Element Plus 自己有测试）

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: 测试框架搭建 + 测试编写，需要中等投入
  - **Skills**: []
    - Vitest 和 Vue Test Utils 是标准工具，无需额外 skill

  **Parallelization**:
  - **Can Run In Parallel**: NO（依赖 Wave 4 的组件拆分完成）
  - **Parallel Group**: Wave 4
  - **Blocks**: F1-F4
  - **Blocked By**: Tasks 11, 12

  **References**:

  **Pattern References**:
  - `server/front/src/App.vue` — `breadcrumbList` computed, `parseDownloadFilename()` 函数
  - `client/front/src/views/DashboardView.vue` — `formatBytes()`, `formatSpeed()`, `taskStatusText()`, `syncProgress()`
  - `client/front/src/views/components/ThemeToggle.vue`（Task 11 创建） — 主题切换组件

  **API/Type References**:
  - `server/front/vite.config.js` — 需参考其 resolve.alias 配置
  - `client/front/vite.config.js` — 同上

  **External References**:
  - Vitest 配置: `https://vitest.dev/config/`
  - Vue Test Utils: `https://test-utils.vuejs.org/`

  **WHY Each Reference Matters**:
  - 工具函数是纯函数，最适合作为首批测试目标
  - ThemeToggle 是新建的组件，验证其切换逻辑正确性

  **Acceptance Criteria**:
  - [ ] `vitest.config.js` 存在于两个项目中
  - [ ] `npx vitest run` 在 server/front 通过（>= 3 个测试）
  - [ ] `npx vitest run` 在 client/front 通过（>= 5 个测试）
  - [ ] 包含 ThemeToggle 组件测试

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: 服务端测试通过
    Tool: Bash
    Steps:
      1. cd server/front && npx vitest run
      2. 检查 exit code 为 0
      3. 检查输出中 "Tests" 行显示 N passed, 0 failed
    Expected Result: 所有测试通过
    Failure Indicators: exit code 非 0 或有 failed tests
    Evidence: .sisyphus/evidence/task-13-server-tests.txt

  Scenario: 客户端测试通过
    Tool: Bash
    Steps:
      1. cd client/front && npx vitest run
      2. 检查 exit code 为 0
      3. 检查输出中 "Tests" 行显示 N passed, 0 failed
    Expected Result: 所有测试通过
    Failure Indicators: exit code 非 0 或有 failed tests
    Evidence: .sisyphus/evidence/task-13-client-tests.txt
  ```

  **Commit**: YES
  - Message: `test(frontend): set up Vitest + add core unit tests`
  - Files: `server/front/vitest.config.js`, `server/front/package.json`, `server/front/src/__tests__/*`, `client/front/vitest.config.js`, `client/front/package.json`, `client/front/src/__tests__/*`
  - Pre-commit: `cd server/front && npx vitest run && cd ../../client/front && npx vitest run`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, check imports, run build). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Build + Code Quality Review** — `unspecified-high`
  Run `npm run build` in both frontends. Review all changed .vue/.css/.js files for: unused imports, hardcoded colors not using CSS variables, emoji icons remaining, Element Plus components imported but not used. Check no `Base` wrapper components were created. Check no TypeScript was added.
  Output: `Server Build [PASS/FAIL] | Client Build [PASS/FAIL] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Visual QA — Both Themes** — `unspecified-high` (+ playwright skill)
  Start both dev servers. For each frontend: take screenshot in light mode, take screenshot in dark mode. Verify theme toggle button exists and works. Verify no FOUC on load. Check all pages: auth, file browser/task list, logs. Check Electron IPC for client. Save screenshots to `.sisyphus/evidence/final-qa/`.
  Output: `Screenshots [N/N captured] | Themes [light/dark both work] | IPC [works/broken] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff. Verify 1:1 — everything in spec was built, nothing beyond spec was built. Check "Must NOT do" compliance: no Pinia, no vue-router in server, no Base wrappers, no TypeScript, no animation libs. Detect cross-task contamination. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **Wave 1 完成**: `feat(frontend): add Element Plus + Lucide + design system infrastructure` — package.json, vite.config.js, style.css (tokens)
- **Wave 2 完成**: `feat(server-front): redesign with Element Plus + Apple HIG style + theme` — server/front/src/*
- **Wave 3 完成**: `feat(client-front): redesign with Element Plus + Apple HIG style + theme` — client/front/src/*
- **Wave 4 完成**: `refactor(frontend): split large components + add Vitest tests` — 拆分后的组件 + 测试文件

---

## Success Criteria

### Verification Commands
```bash
# 构建验证
cd server/front && npm run build          # Expected: 成功，无错误
cd client/front && npm run build          # Expected: 成功，无错误

# 测试验证
cd server/front && npx vitest run         # Expected: 所有测试通过
cd client/front && npx vitest run         # Expected: 所有测试通过

# 开发模式启动
cd server/front && npm run dev            # Expected: http://localhost:5173 正常加载
cd client/front && npm run dev            # Expected: Electron 窗口正常显示

# 禁止项检查
grep -r "import.*Pinia" server/front/src client/front/src  # Expected: 无结果
grep -r "createWebHistory" client/front/src/router         # Expected: 无结果（必须用 hash）
grep -r "📁\|📄" server/front/src client/front/src          # Expected: 无结果（emoji 已替换）
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] Both frontends build successfully
- [ ] Light/dark theme toggle works in both frontends
- [ ] Element Plus zh-CN locale configured
- [ ] No emoji icons remain
- [ ] No FOUC on page load
- [ ] Electron IPC still works
- [ ] macOS build pipeline still works
