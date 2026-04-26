# 测试与 API 规范

> 本文件定义测试要求和 API 文档标准。
> 测试方法论（TDD 流程、覆盖率标准、Go 测试模式）→ 参见 `rules/testing.md`
> 完整 API 端点文档 → 参见 `instructions/api-reference.md`

## 现状

项目零测试覆盖（已确认为严重问题）。新增代码必须带测试。

## 测试要求

### 必须测试

| 模块 | 测试重点 |
|------|----------|
| `resolveUserPath()` / `CleanRelativePath()` | 路径穿越攻击：`../`、符号链接、空字节、超长路径 |
| 认证流程 | 注册校验、登录校验、token 签发/过期/无效、并发注册 |
| 同步引擎 | 快照比对（新增/修改/删除/未变化）、增量跳过逻辑、冲突处理 |
| TaskStore | CRUD + 并发读写 + JSON 持久化往返 |
| 文件上传 | 大小限制、类型过滤、冲突重命名 |

### 测试风格

- 用标准库 `testing`，断言用 `t.Fatal` / `t.Errorf`
- 表驱动测试优先：
  ```go
  func TestResolveUserPath(t *testing.T) {
      tests := []struct {
          name    string
          input   string
          want    string
          wantErr bool
      }{
          {"normal path", "docs/file.txt", "docs/file.txt", false},
          {"traversal", "../../etc/passwd", "", true},
      }
      for _, tt := range tests {
          t.Run(tt.name, func(t *testing.T) {
              // ...
          })
      }
  }
  ```
- 测试文件与源文件同目录：`file_test.go` 放 `file.go` 旁边
- Mock 依赖通过接口注入，不 mock 具体实现

### 不允许

- 不跳过失败测试（`t.Skip` 仅用于已知环境限制）
- 不为提高覆盖率写无意义测试
- 不在测试中硬编码端口或文件路径，用 `t.TempDir()`

---

# API 文档规范

> 实际 API 端点文档 → 参见 `instructions/api-reference.md`（服务端 + 客户端全部端点）
> 本节定义 API 文档的**编写标准**，不包含具体端点内容。

## OpenAPI 规范

所有 API 端点必须有 OpenAPI 3.0 文档，放在 `docs/api.yaml`。

### 文档要求

- 每个端点必须包含：`summary`、`parameters`、`requestBody`（如有）、`responses`（至少 200 + 400 + 401）
- 认证方式统一描述：`security: Bearer <token>`
- 示例请求/响应必须有 `example` 字段

### 响应格式

统一 JSON 结构：

```
成功：{ "message": "操作描述", "data": {...} }
列表：{ "items": [...], "total": 100 }
错误：{ "message": "中文错误描述" }
```

状态码严格限定：200 / 201 / 204 / 400 / 401 / 404 / 409 / 500 / 502。

### 端点命名

- 资源用复数名词：`/tasks`、`/files`
- 动作用动词：`/tasks/:id/sync`、`/tasks/:id/auto/start`
- 查询用 `?path=` 传参，路径参数用 `/:id`
