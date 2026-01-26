# Tool Use 架构设计方案

> **目标**：将当前的多阶段处理流程迁移到 LLM Tool Use 模式，实现更高的灵活性和可扩展性。
>
> **原则**：
> - 每一步有且仅有一项 Tool 调用
> - 充分解耦，各 Tool 独立可测
> - 平稳迁移，两种模式共存
> - 可扩展，易于添加新 Tool

---

## 一、当前架构 vs Tool Use 架构

### 1.1 当前架构（硬编码流水线）

```
┌─────────────────────────────────────────────────────────────┐
│  用户请求 (音频 + 任务类型 + 目标语言)                        │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  Processor.BuildLLMRequest() - 硬编码决策逻辑               │
│                                                             │
│  if task == transcribe && !correction:                      │
│      → 直接 ASR                                             │
│  elif task == translate && correction.merge:                │
│      → ASR → LLM(纠错+翻译)                                 │
│  elif task == translate && !correction.merge:               │
│      → ASR → LLM(纠错) → LLM(翻译)                          │
│                                                             │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  响应                                                        │
└─────────────────────────────────────────────────────────────┘
```

**问题**：
- 流程逻辑硬编码在 Processor 中
- 添加新步骤需要修改核心代码
- 任务组合灵活性有限

### 1.2 Tool Use 架构（LLM 驱动的流水线）

```
┌─────────────────────────────────────────────────────────────┐
│  用户请求 (音频 + 任务描述)                                  │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  Pipeline Orchestrator (编排器)                             │
│                                                             │
│  Mode A: 规则引擎 (静态流水线)                              │
│    → 根据 task 类型预定义 tool 序列                         │
│                                                             │
│  Mode B: LLM Agent (动态流水线)                             │
│    → LLM 自主决定调用哪些 tools 及顺序                      │
│                                                             │
└─────────────────────────────────────────────────────────────┘
                            │
         ┌──────────────────┼──────────────────┐
         ▼                  ▼                  ▼
   ┌───────────┐     ┌───────────┐     ┌───────────┐
   │ Tool: ASR │     │Tool:Correct│    │Tool:Translate│
   │           │     │           │     │              │
   │ 输入:audio│     │ 输入:text │     │ 输入:text    │
   │ 输出:text │     │ 输出:text │     │ 输出:map     │
   └───────────┘     └───────────┘     └───────────┘
         │                  │                  │
         └──────────────────┼──────────────────┘
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  响应聚合器 (Response Aggregator)                           │
└─────────────────────────────────────────────────────────────┘
```

---

## 二、核心设计

### 2.1 Tool 抽象接口

```go
// internal/core/tool/tool.go

package tool

import "context"

// Tool 定义了单个可执行工具的接口
type Tool interface {
    // Name 返回工具的唯一标识符
    Name() string
    
    // Description 返回工具的描述（用于 LLM 选择）
    Description() string
    
    // Schema 返回工具的 JSON Schema（输入参数定义）
    Schema() map[string]interface{}
    
    // Execute 执行工具并返回结果
    Execute(ctx context.Context, input Input) (Output, error)
    
    // Validate 验证输入参数
    Validate(input Input) error
}

// Input 工具输入
type Input struct {
    // 通用数据载体
    Data     map[string]interface{} `json:"data"`
    
    // 上下文传递（前一步的输出可作为后一步的输入）
    Context  *PipelineContext       `json:"context,omitempty"`
}

// Output 工具输出
type Output struct {
    // 结果数据
    Result   map[string]interface{} `json:"result"`
    
    // 元数据（耗时、模型等）
    Metadata map[string]interface{} `json:"metadata,omitempty"`
    
    // 错误信息（如有）
    Error    string                 `json:"error,omitempty"`
}

// PipelineContext 流水线上下文，跨 Tool 传递数据
type PipelineContext struct {
    RequestID       string                   `json:"request_id"`
    OriginalRequest map[string]interface{}   `json:"original_request"`
    StepOutputs     map[string]Output        `json:"step_outputs"` // 各步骤输出
    Dictionary      []DictionaryTerm         `json:"dictionary,omitempty"`
}
```

### 2.2 具体 Tool 实现

#### Tool 1: ASR (语音转文字)

```go
// internal/core/tool/asr_tool.go

package tool

type ASRTool struct {
    asrManager *asr.Manager
    converter  *audio.AudioConverter
    logger     *logrus.Logger
}

func (t *ASRTool) Name() string {
    return "asr"
}

func (t *ASRTool) Description() string {
    return "将音频转换为文本。支持 wav, mp3, opus, m4a, flac 格式。"
}

func (t *ASRTool) Schema() map[string]interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "audio": map[string]interface{}{
                "type":        "string",
                "description": "Base64 编码的音频数据",
            },
            "audio_format": map[string]interface{}{
                "type":        "string",
                "enum":        []string{"wav", "mp3", "opus", "m4a", "flac"},
                "description": "音频格式",
            },
            "language_hint": map[string]interface{}{
                "type":        "string",
                "description": "可选的源语言提示",
            },
        },
        "required": []string{"audio", "audio_format"},
    }
}

func (t *ASRTool) Execute(ctx context.Context, input Input) (Output, error) {
    // 1. 提取参数
    audioData := input.Data["audio"].([]byte)
    audioFormat := input.Data["audio_format"].(string)
    
    // 2. 转换格式（如果需要）
    if t.converter.IsConversionNeeded(audioFormat) {
        audioData, _ = t.converter.ConvertToWAV(audioData, audioFormat)
        audioFormat = "wav"
    }
    
    // 3. 调用 ASR
    resp, err := t.asrManager.Transcribe(ctx, &asr.ASRRequest{
        Audio:       audioData,
        AudioFormat: audioFormat,
    })
    if err != nil {
        return Output{Error: err.Error()}, err
    }
    
    // 4. 返回结果
    return Output{
        Result: map[string]interface{}{
            "text":              resp.Text,
            "detected_language": resp.DetectedLanguage,
            "duration":          resp.Duration,
        },
        Metadata: map[string]interface{}{
            "tool":           "asr",
            "processing_ms":  time.Since(startTime).Milliseconds(),
        },
    }, nil
}
```

#### Tool 2: Correct (文本纠错)

```go
// internal/core/tool/correct_tool.go

type CorrectTool struct {
    llmManager   *llm.Manager
    promptEngine *prompt.Engine
    logger       *logrus.Logger
}

func (t *CorrectTool) Name() string {
    return "correct"
}

func (t *CorrectTool) Description() string {
    return "修正语音识别文本中的错误，包括同音字、错别字和标点问题。可使用用户词典。"
}

func (t *CorrectTool) Schema() map[string]interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "text": map[string]interface{}{
                "type":        "string",
                "description": "需要纠错的文本",
            },
            "dictionary": map[string]interface{}{
                "type":        "array",
                "description": "用户词典，包含术语及其可能的误识别形式",
                "items": map[string]interface{}{
                    "type": "object",
                    "properties": map[string]interface{}{
                        "term":    map[string]string{"type": "string"},
                        "aliases": map[string]interface{}{"type": "array", "items": map[string]string{"type": "string"}},
                    },
                },
            },
        },
        "required": []string{"text"},
    }
}

func (t *CorrectTool) Execute(ctx context.Context, input Input) (Output, error) {
    text := input.Data["text"].(string)
    dictionary := input.Context.Dictionary // 从上下文获取词典
    
    // 构建 prompt
    promptObj, _ := t.promptEngine.BuildTextCorrectPrompt(ctx, text, dictionary)
    
    // 调用 LLM
    resp, err := t.llmManager.ProcessWithTimeout(ctx, &llm.LLMRequest{
        SystemPrompt: promptObj.System,
        UserPrompt:   promptObj.User,
    })
    if err != nil {
        return Output{Error: err.Error()}, err
    }
    
    // 解析结果
    parsed, _ := t.promptEngine.ParseResponse(resp.Content)
    
    return Output{
        Result: map[string]interface{}{
            "corrected_text": parsed.CorrectedText,
            "original_text":  text,
        },
        Metadata: map[string]interface{}{
            "tool":   "correct",
            "model":  resp.Model,
            "tokens": resp.TotalTokens,
        },
    }, nil
}
```

#### Tool 3: Translate (文本翻译)

```go
// internal/core/tool/translate_tool.go

type TranslateTool struct {
    llmManager   *llm.Manager
    promptEngine *prompt.Engine
    logger       *logrus.Logger
}

func (t *TranslateTool) Name() string {
    return "translate"
}

func (t *TranslateTool) Description() string {
    return "将文本翻译成一个或多个目标语言。"
}

func (t *TranslateTool) Schema() map[string]interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "text": map[string]interface{}{
                "type":        "string",
                "description": "需要翻译的文本",
            },
            "target_languages": map[string]interface{}{
                "type":        "array",
                "description": "目标语言代码列表",
                "items":       map[string]string{"type": "string"},
            },
        },
        "required": []string{"text", "target_languages"},
    }
}

func (t *TranslateTool) Execute(ctx context.Context, input Input) (Output, error) {
    text := input.Data["text"].(string)
    targetLangs := input.Data["target_languages"].([]string)
    
    // 构建 prompt
    promptObj, _ := t.promptEngine.BuildTextPrompt(ctx, prompt.PromptRequest{
        Task:            prompt.TaskTranslate,
        TargetLanguages: targetLangs,
        Variables:       map[string]interface{}{"source_text": text},
    })
    
    // 调用 LLM
    resp, err := t.llmManager.ProcessWithTimeout(ctx, &llm.LLMRequest{
        SystemPrompt: promptObj.System,
        UserPrompt:   promptObj.User,
    })
    if err != nil {
        return Output{Error: err.Error()}, err
    }
    
    // 解析结果
    parsed, _ := t.promptEngine.ParseResponse(resp.Content)
    
    return Output{
        Result: map[string]interface{}{
            "translations": parsed.Sections,
            "source_text":  text,
        },
        Metadata: map[string]interface{}{
            "tool":   "translate",
            "model":  resp.Model,
            "tokens": resp.TotalTokens,
        },
    }, nil
}
```

#### Tool 4: CorrectAndTranslate (合并工具)

```go
// internal/core/tool/correct_translate_tool.go

type CorrectAndTranslateTool struct {
    llmManager   *llm.Manager
    promptEngine *prompt.Engine
    logger       *logrus.Logger
}

func (t *CorrectAndTranslateTool) Name() string {
    return "correct_and_translate"
}

func (t *CorrectAndTranslateTool) Description() string {
    return "在一次调用中同时完成文本纠错和翻译，减少延迟。"
}

// ... Execute 使用 text_correct_translate 模板
```

---

## 三、流水线编排器

### 3.1 Pipeline 定义

```go
// internal/core/pipeline/pipeline.go

package pipeline

// Step 定义流水线中的一个步骤
type Step struct {
    ToolName    string                 `json:"tool_name"`
    InputMap    map[string]string      `json:"input_map"`    // 输入映射规则
    OutputKey   string                 `json:"output_key"`   // 输出存储键名
    Condition   string                 `json:"condition"`    // 可选：执行条件
}

// Pipeline 定义完整的流水线
type Pipeline struct {
    Name        string   `json:"name"`
    Description string   `json:"description"`
    Steps       []Step   `json:"steps"`
}

// 预定义流水线
var PredefinedPipelines = map[string]Pipeline{
    "transcribe": {
        Name:        "transcribe",
        Description: "仅转录音频",
        Steps: []Step{
            {ToolName: "asr", InputMap: map[string]string{"audio": "request.audio", "audio_format": "request.audio_format"}, OutputKey: "asr_result"},
        },
    },
    "transcribe_with_correction": {
        Name:        "transcribe_with_correction",
        Description: "转录并纠错",
        Steps: []Step{
            {ToolName: "asr", InputMap: map[string]string{"audio": "request.audio", "audio_format": "request.audio_format"}, OutputKey: "asr_result"},
            {ToolName: "correct", InputMap: map[string]string{"text": "asr_result.text"}, OutputKey: "correct_result"},
        },
    },
    "translate": {
        Name:        "translate",
        Description: "转录、纠错并翻译",
        Steps: []Step{
            {ToolName: "asr", InputMap: map[string]string{"audio": "request.audio", "audio_format": "request.audio_format"}, OutputKey: "asr_result"},
            {ToolName: "correct_and_translate", InputMap: map[string]string{"text": "asr_result.text", "target_languages": "request.target_languages"}, OutputKey: "final_result"},
        },
    },
    "translate_separated": {
        Name:        "translate_separated",
        Description: "转录、纠错、翻译（分离模式）",
        Steps: []Step{
            {ToolName: "asr", InputMap: map[string]string{"audio": "request.audio"}, OutputKey: "asr_result"},
            {ToolName: "correct", InputMap: map[string]string{"text": "asr_result.text"}, OutputKey: "correct_result"},
            {ToolName: "translate", InputMap: map[string]string{"text": "correct_result.corrected_text", "target_languages": "request.target_languages"}, OutputKey: "translate_result"},
        },
    },
}
```

### 3.2 Pipeline Executor

```go
// internal/core/pipeline/executor.go

package pipeline

type Executor struct {
    registry  *ToolRegistry
    logger    *logrus.Logger
}

func NewExecutor(registry *ToolRegistry, logger *logrus.Logger) *Executor {
    return &Executor{registry: registry, logger: logger}
}

// Execute 执行流水线
func (e *Executor) Execute(ctx context.Context, pipeline Pipeline, request map[string]interface{}) (*ExecutionResult, error) {
    pipelineCtx := &tool.PipelineContext{
        RequestID:       generateRequestID(),
        OriginalRequest: request,
        StepOutputs:     make(map[string]tool.Output),
    }
    
    result := &ExecutionResult{
        RequestID: pipelineCtx.RequestID,
        Steps:     make([]StepResult, 0, len(pipeline.Steps)),
    }
    
    for i, step := range pipeline.Steps {
        // 1. 获取 Tool
        t, ok := e.registry.Get(step.ToolName)
        if !ok {
            return nil, fmt.Errorf("unknown tool: %s", step.ToolName)
        }
        
        // 2. 构建输入
        input := e.buildInput(step.InputMap, pipelineCtx)
        input.Context = pipelineCtx
        
        // 3. 执行 Tool
        stepStart := time.Now()
        output, err := t.Execute(ctx, input)
        stepDuration := time.Since(stepStart)
        
        if err != nil {
            return nil, fmt.Errorf("step %d (%s) failed: %w", i, step.ToolName, err)
        }
        
        // 4. 存储输出
        pipelineCtx.StepOutputs[step.OutputKey] = output
        
        // 5. 记录步骤结果
        result.Steps = append(result.Steps, StepResult{
            StepIndex:  i,
            ToolName:   step.ToolName,
            Duration:   stepDuration,
            Output:     output,
        })
        
        e.logger.WithFields(logrus.Fields{
            "step":      i,
            "tool":      step.ToolName,
            "duration":  stepDuration.Milliseconds(),
        }).Debug("Pipeline step completed")
    }
    
    result.FinalOutput = e.aggregateOutputs(pipelineCtx, pipeline)
    return result, nil
}

// buildInput 根据映射规则构建输入
func (e *Executor) buildInput(inputMap map[string]string, ctx *PipelineContext) tool.Input {
    data := make(map[string]interface{})
    
    for key, path := range inputMap {
        value := e.resolvePath(path, ctx)
        data[key] = value
    }
    
    return tool.Input{Data: data}
}

// resolvePath 解析路径表达式，如 "asr_result.text" 或 "request.audio"
func (e *Executor) resolvePath(path string, ctx *PipelineContext) interface{} {
    parts := strings.SplitN(path, ".", 2)
    if len(parts) < 2 {
        return nil
    }
    
    source := parts[0]
    field := parts[1]
    
    switch source {
    case "request":
        return ctx.OriginalRequest[field]
    default:
        if output, ok := ctx.StepOutputs[source]; ok {
            return output.Result[field]
        }
    }
    return nil
}
```

### 3.3 Tool Registry

```go
// internal/core/pipeline/registry.go

package pipeline

type ToolRegistry struct {
    tools map[string]tool.Tool
    mu    sync.RWMutex
}

func NewToolRegistry() *ToolRegistry {
    return &ToolRegistry{tools: make(map[string]tool.Tool)}
}

func (r *ToolRegistry) Register(t tool.Tool) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.tools[t.Name()] = t
}

func (r *ToolRegistry) Get(name string) (tool.Tool, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    t, ok := r.tools[name]
    return t, ok
}

func (r *ToolRegistry) List() []tool.Tool {
    r.mu.RLock()
    defer r.mu.RUnlock()
    list := make([]tool.Tool, 0, len(r.tools))
    for _, t := range r.tools {
        list = append(list, t)
    }
    return list
}

// GetToolDefinitions 返回所有工具的定义（用于 LLM Tool Use）
func (r *ToolRegistry) GetToolDefinitions() []map[string]interface{} {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    definitions := make([]map[string]interface{}, 0, len(r.tools))
    for _, t := range r.tools {
        definitions = append(definitions, map[string]interface{}{
            "type": "function",
            "function": map[string]interface{}{
                "name":        t.Name(),
                "description": t.Description(),
                "parameters":  t.Schema(),
            },
        })
    }
    return definitions
}
```

---

## 四、双模式共存设计

### 4.1 配置项

```yaml
# config/config.yaml

pipeline:
  mode: "static"  # "static" | "tool_use" | "auto"
  
  # 静态模式配置（当前架构）
  static:
    transcribe: "transcribe"
    transcribe_with_correction: "transcribe_with_correction"
    translate: "translate"
    translate_separated: "translate_separated"
  
  # Tool Use 模式配置
  tool_use:
    enabled: false  # 开启后使用 LLM Agent 决策
    max_steps: 5    # 最大执行步数
    model: "qwen3"  # 用于决策的 LLM 模型
```

### 4.2 处理器适配层

```go
// internal/core/audio/processor_v2.go

type ProcessorV2 struct {
    // 现有依赖
    asrManager   *asr.Manager
    llmManager   *llm.Manager
    promptEngine *prompt.Engine
    
    // Tool Use 新增
    pipelineExecutor *pipeline.Executor
    pipelineMode     string
}

func (p *ProcessorV2) Process(ctx context.Context, req ProcessRequest) (*ProcessResponse, error) {
    switch p.pipelineMode {
    case "static":
        // 使用现有逻辑（保持兼容）
        return p.processStatic(ctx, req)
    
    case "tool_use":
        // 使用 Pipeline Executor
        return p.processWithPipeline(ctx, req)
    
    case "auto":
        // 根据请求复杂度自动选择
        if p.isComplexRequest(req) {
            return p.processWithPipeline(ctx, req)
        }
        return p.processStatic(ctx, req)
    
    default:
        return p.processStatic(ctx, req)
    }
}

func (p *ProcessorV2) processWithPipeline(ctx context.Context, req ProcessRequest) (*ProcessResponse, error) {
    // 1. 选择流水线
    pipelineName := p.selectPipelineForRequest(req)
    pipeline := pipeline.PredefinedPipelines[pipelineName]
    
    // 2. 构建请求数据
    requestData := map[string]interface{}{
        "audio":            req.Audio,
        "audio_format":     req.AudioFormat,
        "target_languages": req.TargetLanguages,
        "dictionary":       req.UserDictionary,
    }
    
    // 3. 执行流水线
    result, err := p.pipelineExecutor.Execute(ctx, pipeline, requestData)
    if err != nil {
        return nil, err
    }
    
    // 4. 构建响应
    return p.buildResponseFromPipelineResult(result, req), nil
}

func (p *ProcessorV2) selectPipelineForRequest(req ProcessRequest) string {
    switch req.Task {
    case prompt.TaskTranscribe:
        if p.correctionEnabled {
            return "transcribe_with_correction"
        }
        return "transcribe"
    case prompt.TaskTranslate:
        if p.correctionMerge {
            return "translate"
        }
        return "translate_separated"
    }
    return "transcribe"
}
```

---

## 五、扩展性设计

### 5.1 添加新 Tool 的步骤

1. **实现 Tool 接口**：
   ```go
   type MyNewTool struct { ... }
   func (t *MyNewTool) Name() string { return "my_new_tool" }
   func (t *MyNewTool) Execute(...) (Output, error) { ... }
   ```

2. **注册到 Registry**：
   ```go
   registry.Register(NewMyNewTool(...))
   ```

3. **定义新的 Pipeline**（可选）：
   ```go
   PredefinedPipelines["my_pipeline"] = Pipeline{
       Steps: []Step{
           {ToolName: "asr", ...},
           {ToolName: "my_new_tool", ...},
       },
   }
   ```

### 5.2 可扩展的 Tool 类型

| Tool 名称 | 功能 | 状态 |
|-----------|------|------|
| `asr` | 语音转文字 | ✅ 核心 |
| `correct` | 文本纠错 | ✅ 核心 |
| `translate` | 文本翻译 | ✅ 核心 |
| `correct_and_translate` | 纠错+翻译合并 | ✅ 核心 |
| `language_detect` | 语言检测 | 🔮 未来 |
| `sentiment_analyze` | 情感分析 | 🔮 未来 |
| `summarize` | 文本摘要 | 🔮 未来 |
| `tts` | 文字转语音 | 🔮 未来 |
| `filter` | 敏感词过滤 | 🔮 未来 |

### 5.3 动态 Pipeline 定义（未来）

支持从配置文件或 API 动态定义 Pipeline：

```yaml
# pipelines/custom_pipeline.yaml
name: vtuber_stream
description: VTuber 直播翻译流水线
steps:
  - tool: asr
    input:
      audio: ${request.audio}
    output: transcription
  
  - tool: correct
    input:
      text: ${transcription.text}
      dictionary: ${request.dictionary}
    output: corrected
  
  - tool: filter
    input:
      text: ${corrected.corrected_text}
    output: filtered
    condition: ${request.enable_filter}
  
  - tool: translate
    input:
      text: ${filtered.text}
      target_languages: [en, ja]
    output: translations
```

---

## 六、迁移计划

### Phase 1：基础设施 (Week 1)

- [ ] 创建 `internal/core/tool/` 包
- [ ] 实现 `Tool` 接口和 `Registry`
- [ ] 实现 `ASRTool`, `CorrectTool`, `TranslateTool`

### Phase 2：Pipeline 引擎 (Week 2)

- [ ] 创建 `internal/core/pipeline/` 包
- [ ] 实现 `Pipeline` 定义和 `Executor`
- [ ] 预定义核心 Pipeline

### Phase 3：集成适配 (Week 3)

- [ ] 创建 `ProcessorV2` 适配层
- [ ] 添加 `pipeline.tool_calling.*` 配置项
- [ ] 确保现有 API 兼容性

### Phase 4：测试验证 (Week 4)

- [ ] 单元测试每个 Tool
- [ ] 集成测试 Pipeline 执行
- [ ] A/B 测试两种模式性能对比

### Phase 5：切换与清理 (Week 5+)

- [ ] 逐步将流量切换到 Tool Use 模式
- [ ] 根据监控结果调优
- [ ] 清理旧代码（可选）

---

## 七、对比分析

| 维度 | 当前架构 | Tool Use 架构 |
|------|----------|---------------|
| **灵活性** | 低（硬编码流程） | 高（可配置流水线） |
| **扩展性** | 差（改核心代码） | 优（注册新 Tool） |
| **可测试性** | 一般 | 高（各 Tool 独立测试） |
| **调试难度** | 一般 | 低（步骤清晰可追踪） |
| **性能** | 较优（无调度开销） | 略有开销（可忽略） |
| **复杂度** | 低 | 中等 |

---

## 八、总结

Tool Use 架构通过以下方式实现平稳迁移：

1. **抽象 Tool 接口**：每个功能模块实现统一接口，独立可测
2. **Pipeline 编排**：静态定义或动态决策执行顺序
3. **统一 Pipeline 模式**：所有请求走 Tool Use Pipeline 执行路径
4. **渐进式迁移**：先实现 Tool，再接入 Pipeline，最后清理旧代码

这种设计为未来扩展（如 LLM Agent 动态编排、自定义 Pipeline、新功能 Tool）奠定了良好基础。
