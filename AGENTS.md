# Lingualink Core - AI Agent Guidelines

> 这是给 AI Coding Assistant (Codex/Claude) 的精简指南。人类开发者请参考 `/docs` 目录。

## 核心架构

```
请求 → API Handler → Pipeline Executor → Tool Chain → 响应
                           ↓
              Tool Registry (asr, correct, translate, correct_translate)
```

**关键设计决策**:
- 所有处理通过 `Tool` + `Pipeline` 编排
- Tool 是最小可执行单元，实现 `tool.Tool` 接口
- Pipeline 是 Tool 的有序组合，定义在 `pipeline/predefined.go`

## 目录结构 (重点)

| 路径 | 职责 | 修改频率 |
|------|------|---------|
| `internal/core/tool/` | Tool 实现 (asr, translate, correct) | 高 |
| `internal/core/pipeline/` | Pipeline 定义和执行器 | 中 |
| `internal/core/prompt/` | Prompt 模板和语言配置 | 中 |
| `internal/core/llm/` | LLM 后端管理、Tool Calling | 低 |
| `internal/core/asr/` | ASR 后端 (Whisper) | 低 |
| `internal/api/handlers/` | HTTP handlers | 中 |
| `config/` | YAML/JSON 配置 | 优先改配置 |

## 快速命令

```bash
# 开发
./manage.sh start|stop|restart|status|logs
go test ./...
gofmt -w .

# 测试 API
curl -s http://localhost:8080/api/v1/health | jq
./test_api.sh
```

## Pipeline 选择逻辑

```
process_audio:
  task=transcribe:
    correction.enabled=true  → transcribe_correct (ASR → Correct)
    correction.enabled=false → transcribe (ASR only)
  task=translate:
    correction.merge=true    → translate_merged (ASR → CorrectTranslate)
    correction.merge=false   → translate_split (ASR → Correct → Translate)
```

## Tool 接口规范

```go
type Tool interface {
    Name() string                                           // 唯一标识
    Description() string                                    // LLM tool calling 描述
    Schema() map[string]interface{}                         // 输入 JSON Schema
    OutputSchema() map[string]interface{}                   // 输出 Schema (LLM tools)
    Validate(input Input) error                             // 输入验证
    Execute(ctx context.Context, input Input) (Output, error) // 执行
}
```

## 配置优先级

```
环境变量 > config/config.yaml > 代码默认值
```

**常用环境变量**:
- `LINGUALINK_CONFIG_FILE`: 配置文件路径
- `LINGUALINK_KEYS_FILE`: API 密钥文件路径
- `SERVER_PORT`: 服务端口

## 添加新功能的模式

### 添加新 Tool
1. 在 `internal/core/tool/` 创建 `xxx_tool.go`
2. 实现 `Tool` 接口
3. 在 `cmd/server/main.go` 注册到 Registry

### 添加新 Pipeline
1. 在 `pipeline/predefined.go` 添加函数
2. 在 `audio/processor.go` 的 `selectPipeline()` 添加选择逻辑

### 添加新语言
1. 编辑 `config/config.yaml` 的 `prompt.languages`
2. 无需改代码

## 关键文件位置

| 需求 | 文件 |
|------|------|
| 修改 API 响应格式 | `audio/processor.go`, `text/processor.go` |
| 修改 Prompt 模板 | `prompt/template.go` |
| 添加 LLM 参数 | `llm/types.go`, `llm/base_backend.go` |
| 修改认证逻辑 | `pkg/auth/auth.go` |
| 添加指标 | `pkg/metrics/metrics.go` |

## 测试约定

- 单元测试: `xxx_test.go` 同目录
- 集成测试: `internal/testutil/integrationtest/`
- 测试音频: `test/test.opus`, `test/test.wav`
- Mock 模式: 设置 `h.llmManager = nil` 会触发配置错误

## 常见错误处理

| 错误码 | 含义 | 处理 |
|--------|------|------|
| `ErrCodeValidation` | 400 请求无效 | 检查必填字段 |
| `ErrCodeAuth` | 401 认证失败 | 检查 X-API-Key |
| `ErrCodeLLM` | 502 LLM 错误 | 检查后端健康 |
| `ErrCodeParsing` | 502 解析失败 | 检查 Prompt/响应格式 |

## 架构文档

详细设计请参考:
- `docs/architecture.md` - 系统架构
- `docs/configuration.md` - 配置详解
- `docs/api.md` - API 参考
