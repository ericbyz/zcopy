---
name: gen-commit
description: "根据 staged changes 生成规范的 commit message。"
---

# 生成 Commit Message

## 流程

```
1. 运行 git diff --cached 查看暂存区变更
2. 分析变更内容
3. 匹配 commit type 和 scope
4. 生成 commit message
5. 用户确认后执行 git commit
```

## Commit Message 格式

```
<type>(<scope>): <description>

[可选 body：为什么改，不是改了什么]
[可选 footer：关联 issue]
```

## Type 列表

| Type | 说明 | 示例 |
|------|------|------|
| feat | 新功能 | feat(auth): add token refresh support |
| fix | Bug 修复 | fix(sync): handle empty directory without panic |
| refactor | 重构（不改行为） | refactor(store): extract interface from TaskStore |
| test | 测试 | test(sync): add snapshot comparison coverage |
| docs | 文档 | docs(api): update sync endpoint descriptions |
| chore | 构建/工具 | chore(deps): upgrade Electron to v30 |
| perf | 性能 | perf(sync): batch file uploads with concurrency limit |

## Scope 列表（ZCopy 项目专用）

| Scope | 覆盖范围 |
|-------|---------|
| sync | 同步引擎（client/backend/sync/） |
| auth | 认证（handlers_auth, auth/） |
| task | 任务管理（handlers_task, store/） |
| fp | File Provider（fileprovider/, front-mac/fileprovider/） |
| ui | 前端（client/front/src/, server/front/src/） |
| server | 服务端（server/backend/） |
| build | 构建打包（scripts/, electron-builder） |
| api | API 接口设计 |

## 规则

```
1. description 用祈使句（"add" 不用 "added"）
2. 不超过 72 字符
3. 改了什么不重要，为什么改才重要
4. 一个 commit 一个关注点
5. 如果 staged changes 涉及多个不相关的改动 → 建议拆分
6. Fix PR 里不要混入重构
```

## 判断 Type 的依据

```
新增文件 + 新逻辑 → feat
修改已有文件 + 修 Bug → fix
修改已有文件 + 不改行为 → refactor
只改测试 → test
只改文档/注释 → docs
改配置/依赖 → chore
改算法/查询 + 有 benchmark → perf
```
