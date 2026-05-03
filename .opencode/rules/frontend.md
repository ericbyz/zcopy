---
title: 前端开发规则
scope: "client/front/src/**/*.{vue,js}, client/front-mac/electron/**/*.js, server/front/src/**/*.{vue,js,css}"
priority: HIGH
---

# 前端开发规则

**改动前端文件时自动生效。**

> 参见 instructions/build-and-packaging.md（打包脚本、构建命令）

---

## Vue 3 组件设计

### 文件结构

```
ZCopy 前端结构（非组件库模式，按功能组织）：
views/
├── DashboardView.vue       # 页面组件
├── LogViewer.vue            # 页面组件
├── SettingsView.vue         # 页面组件
└── components/              # 可复用组件
    ├── AuthCard.vue         # 认证卡片
    ├── TaskCard.vue         # 任务卡片
    ├── TaskForm.vue         # 任务表单
    ├── TaskList.vue         # 任务列表
    ├── RemoteFolderPicker.vue  # 远程目录选择
    └── PlatformInfo.vue     # 平台信息
```

### 命名规范

```
组件文件：PascalCase（AuthCard.vue、TaskList.vue）
Props 接口：在 <script setup> 中用 defineProps 定义
事件处理：handle{Event}（handleSubmit、handleSync）
Emits：on{Event}（onTaskCreated、onSyncComplete）
CSS 类名：kebab-case（.task-card、.sync-status）
```

### Composition API 规范

```vue
<!-- ✅ 正确：<script setup> + Composition API -->
<script setup>
import { ref, onMounted } from 'vue'

const tasks = ref([])
const loading = ref(false)

async function fetchTasks() {
  loading.value = true
  try {
    const res = await axios.get('/tasks')
    tasks.value = res.data.items
  } finally {
    loading.value = false
  }
}

onMounted(fetchTasks)
</script>

<!-- ❌ 错误：Options API -->
<script>
export default {
  data() { return { tasks: [] } },
  methods: { async fetchTasks() { ... } }
}
</script>
```

---

## 状态管理

```
ZCopy 不使用 Vuex / Pinia，采用以下策略：

                ┌─────────────┐
                │   URL 状态   │ ← 路由参数（router/index.js）
                └──────┬──────┘
                       │
                ┌──────┴──────┐
                │ provide/inject │ ← 跨组件共享（认证状态等）
                └──────┬──────┘
                       │
                ┌──────┴──────┐
                │  组件状态    │ ← ref / reactive（组件内）
                └─────────────┘

规则：
- UI 状态放组件内（ref / reactive）
- 共享状态通过 provide/inject 向下传递
- 不使用全局事件总线
- 异步操作有明确的 loading / error / success 状态

禁止：
- props drilling 超过 2 层
- 在 watch 里直接修改被 watch 的数据导致无限循环
- 在组件外直接修改共享状态
```

---

## Electron IPC 模式

```
渲染层（Vue）  ←→  preload.js  ←→  main.js（Electron 主进程）
     │                                    │
     │  window.desktopApi.pickFolder()    │
     │  ─────────────────────────────────►│
     │         ipcRenderer.invoke()       │
     │                                    │  dialog.showOpenDialog()
     │          返回选择路径              │
     │  ◄─────────────────────────────────│

规则：
- preload.js 只暴露必要的 IPC 方法（contextBridge.exposeInMainWorld）
- 主进程通过 ipcMain.handle 处理请求
- 禁止在渲染层直接使用 Node.js API
- 禁止在 preload 中暴露 fs、child_process 等危险模块
```

---

## 样式规范

```
原则：
- ZCopy 使用深色主题 + 玻璃态面板（参见 server/front/src/style.css）
- 颜色、间距、字号保持与现有组件一致
- 响应式用相对单位（rem、em、%）+ media queries
- z-index 层级：保持在现有范围内（不要随意定值）

禁止：
- !important
- 内联样式（除非动态计算的值）
- 超过 3 层的嵌套选择器
- 随意引入新的 CSS 框架（项目未使用 Tailwind 等工具类框架）
```

---

## 可访问性（a11y）

```
必须：
- 图片有 alt 属性
- 按钮和链接有描述性文本
- 表单 label 关联正确
- 键盘可操作（tab order、focus 样式）
- 颜色对比度 >= 4.5:1（深色主题特别注意）
```

---

## 性能

```
必须：
- 列表使用 :key（用稳定 ID，不要用 index）
- 大列表考虑虚拟滚动
- 路由懒加载（const Dashboard = () => import('./views/DashboardView.vue')）
- 避免不必要的组件重渲染

禁止：
- 在 render 里创建新对象/数组/函数（导致子组件不必要的更新）
- 无限滚动的 DOM 不回收
- 在 onMounted 中做大量同步计算
```

---

## 常见踩坑点

| 场景 | 陷阱 | 正确做法 |
|------|------|---------|
| watch | 缺少 immediate 或 deep 配置 | 分析是否需要立即执行/深度监听 |
| 列表渲染 | 用 index 做 :key | 用任务的 ID 字段 |
| 异步操作 | 忘记处理 loading 和 error 状态 | 每个异步操作都维护三态 |
| Electron IPC | 渲染层直接调 Node API | 通过 preload.js 暴露 |
| 主题切换 | 硬编码颜色值 | 使用 CSS 变量 |
| Element Plus | 忘记设置中文 locale | 使用 ElConfigProvider + zhCn |
