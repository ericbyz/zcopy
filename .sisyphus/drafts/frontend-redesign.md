# Draft: 前端设计更新 - 简约风格 + 明暗主题

## 现状分析

### 服务端前端 (server/front/)
- **技术栈**: Vue 3 (Composition API) + Vite，纯手写 CSS，零 UI 框架
- **组件**: 单文件 App.vue (~372行) + 1个子组件 LogViewer.vue
- **样式**: style.css (~264行)，暗色玻璃态风格
- **配色**: 深蓝底 `#0f172a`，渐变光斑装饰，蓝色系按钮 `#3b82f6→#4338ca`
- **当前无明暗切换**，纯暗色模式

### 客户端前端 (client/front/)
- **技术栈**: Vue 3 + Vue Router + Vite，纯手写 CSS，零 UI 框架
- **组件**: App.vue (~45行骨架) + views/DashboardView.vue (~663行) + LogViewer.vue
- **样式**: style.css (~408行)，风格与服务端一致（暗色玻璃态）
- **配色**: 与服务端基本一致，深蓝底 + 蓝紫色渐变按钮
- **当前无明暗切换**，纯暗色模式

### 两个前端共享特征
- 无 CSS 变量系统（颜色硬编码在各处）
- 无设计系统 / 组件库
- emoji 作为文件图标（📁📄）
- 响应式已有基础（@media 断点）
- 风格一致但代码完全独立（两份 style.css）

## 用户需求
- **简约** (minimalist) 设计
- **白天黑夜模式** (light/dark mode toggle)

## 已确认的设计方向

### 用户选择（全部确认）
1. **设计风格**: Apple HIG 风格 — 大量留白、圆角、微妙阴影、半透明毛玻璃
2. **UI 组件库**: Element Plus（企业级、中文生态好、内置 dark mode、CSS 变量主题）
3. **图标库**: Lucide Icons（简约线条风格，接近 Apple SF Symbols）
4. **设计统一性**: 两个前端共享统一设计系统
5. **主题切换**: 跟随系统 + 手动切换（localStorage 持久化）
6. **改造范围**: 视觉层 + 组件拆分（拆分大组件如 DashboardView.vue 663行）

### 探索发现（关键约束）
- macOS 客户端 (front-mac) 没有自己的 Vue 代码，直接消费 client/front/dist
- 因此只需改 **两套代码**: server/front + client/front，macOS 自动受益
- 当前 emoji 图标系统过于简陋，需要引入图标库
- 两个前端各自有独立但高度重复的 CSS（~264行 + ~408行）
- 没有 CSS 变量系统，颜色全部硬编码
- 组件结构臃肿（DashboardView.vue 663行），但本次重点是视觉层

### 需要组件库的理由
- 按钮样式变体太多（primary/ghost/danger/link/block/active/disabled）
- 表单控件需要统一（input/checkbox/select/tab）
- 需要 Toast 通知系统（替代当前 inline message）
- 模态框系统（替代当前手写的 picker-mask）
- 主题切换需要组件库原生支持

## 备选 Vue 3 组件库评估

| 库 | 主题定制 | Dark/Light | Apple HIG 适配度 | 中文支持 | 体积 | 维护状态 |
|---|---|---|---|---|---|---|
| Naive UI | 极好（CSS vars） | 内置 | 中高（可定制） | 原生中文 | 中等 (~70KB gzip) | 活跃 |
| Element Plus | 好（CSS vars） | 内置 | 中（更 Material） | 原生中文 | 较大 (~80KB gzip) | 活跃 |
| PrimeVue | 极好 | 内置 | 中高 | 好 | 可按需 | 活跃 |
| Vuetify 3 | 好 | 内置 | 低（强 Material） | 好 | 大 | 活跃 |

## 待讨论
1. ~~具体选哪个组件库？~~ → Element Plus
2. ~~图标库偏好？~~ → Lucide Icons
3. ~~是否需要组件拆分~~ → 是，视觉层 + 组件拆分
4. ~~测试策略~~ → 搭建测试框架(Vitest + Vue Test Utils) + 核心组件单元测试

## Test Strategy Decision
- **Infrastructure exists**: NO（需从零搭建）
- **Automated tests**: YES（先搭建框架，再为核心组件写单元测试）
- **Framework**: Vitest + @vue/test-utils
- **Test focus**: 主题切换逻辑、组件 props/events、表单验证、设计 token 正确性

## Scope Boundaries
- INCLUDE: 服务端前端 + 客户端前端的视觉重构 + 明暗主题 + 统一设计系统
- INCLUDE: 引入 Element Plus + Lucide Icons
- INCLUDE: 组件拆分（DashboardView、App.vue 等）
- INCLUDE: 搭建测试框架 + 核心组件单元测试
- EXCLUDE: 后端 Go 代码不变
- EXCLUDE: API 接口不变
- EXCLUDE: Electron 主进程代码不变
- EXCLUDE: 不新增功能，纯 UI/UX 改造
