---
name: health-check
description: "检查 ZCopy 项目健康度：测试覆盖、代码质量、依赖状态、构建一致性。"
---

# 项目健康度检查

## 检查维度

### 1. 测试健康度

```
- [ ] Go 后端测试通过（client/backend + server/backend）
- [ ] 前端测试通过（如果有）
- [ ] 测试覆盖率达标（参见 .opencode/agents/quality.md）
- [ ] 无被跳过的测试（除非有 TODO 注释）
- [ ] 测试文件镜像源码结构（*_test.go）
- [ ] 关键模块有测试（sync、auth、store、路径安全）
```

运行命令：
```bash
cd client/backend && go test ./... -v -cover
cd server/backend && go test ./... -v -cover
```

### 2. 代码质量

```
- [ ] go vet 通过（0 warnings）
- [ ] 无 panic 在库代码中
- [ ] 无 log.Fatalf 在工具函数中
- [ ] 无 as any / @ts-ignore（前端）
- [ ] 无空 catch 块
- [ ] 文件不超过 300 行（检查 main.go 等已知技术债务）
- [ ] 函数不超过 50 行
```

运行命令：
```bash
cd client/backend && go vet ./...
cd server/backend && go vet ./...
```

### 3. 构建健康度

```
- [ ] 客户端 Go 后端编译通过（两个平台）
- [ ] 服务端 Go 后端编译通过
- [ ] 前端构建通过（client/front + server/front）
- [ ] macOS: Swift 组件编译通过
- [ ] 打包脚本执行成功（npm run dist）
```

运行命令：
```bash
cd client/front && npm run build:backend
cd server/backend && go build -o /dev/null .
cd server/front && npm run build
cd client/front-mac && npm run build:renderer
```

### 4. 依赖状态

```
- [ ] Go 模块依赖无安全漏洞
- [ ] npm 依赖无已知漏洞
- [ ] go.sum 和 go.mod 一致
- [ ] package-lock.json 和 package.json 一致
- [ ] 无未使用的依赖
```

运行命令：
```bash
cd client/backend && go mod tidy && go mod verify
cd server/backend && go mod tidy && go mod verify
cd client/front && npm audit
cd server/front && npm audit
```

### 5. 配置一致性

```
- [ ] .opencode/rules/ 和实际实践一致
- [ ] .opencode/instructions/ 内容和代码匹配
- [ ] AGENTS.md 技术栈描述和实际一致
- [ ] config.yaml 配置项和代码使用一致
- [ ] 环境变量覆盖机制有效
```

### 6. 文档同步

```
- [ ] AGENTS.md API 接口描述和路由代码一致
- [ ] README 安装/运行步骤有效
- [ ] .opencode/ 文件交叉引用路径正确
- [ ] 已知限制列表更新
```

## 输出模板

```markdown
## ZCopy 项目健康度报告

**日期**: YYYY-MM-DD
**总体评分**: 🟢 健康 / 🟡 需关注 / 🔴 需修复

### 测试 🟢/🟡/🔴
- 覆盖率: X%
- 通过/总数: N/M
- 问题: [列出]

### 代码质量 🟢/🟡/🔴
- go vet: [结果]
- 已知技术债务: [列出]
- 问题: [列出]

### 构建 🟢/🟡/🔴
- 客户端: [结果]
- 服务端: [结果]
- 问题: [列出]

### 依赖 🟢/🟡/🔴
- 安全漏洞: N
- 问题: [列出]

### 配置 🟢/🟡/🔴
- 问题: [列出]

### 文档 🟢/🟡/🔴
- 问题: [列出]

### 建议操作（按优先级）
1. [最紧急]
2. [次紧急]
3. ...
```

## 使用场景

```
1. Sprint 结束时跑一次
2. 大版本发布前跑一次
3. 发现质量下滑时跑一次
4. 新人加入项目时跑一次（作为 baseline）
5. 大规模重构前跑一次（确认基线）
```

## 已知问题（长期追踪）

```
⚠️ 零测试覆盖（严重）
⚠️ client/backend/main.go 1506 行（技术债务）
⚠️ AppState 6 种职责（技术债务）
⚠️ File Provider REST 端点无认证
⚠️ App Group entitlements 空数组
⚠️ CORS 回显任意 Origin
⚠️ Token 仅存内存，重启丢失
```
