# Lingualink Core 代码质量提升计划

本文档定义了 Codex/AI Agent 可执行的代码质量提升任务。每个任务都是独立的，可以并行执行。

---

## 执行指南

### 优先级说明
- 🔴 **P0 - Critical**: 必须优先完成，影响核心功能
- 🟠 **P1 - High**: 重要改进，显著提升代码质量
- 🟡 **P2 - Medium**: 有价值的改进
- 🟢 **P3 - Low**: 锦上添花的改进

### 任务状态
- `[ ]` 待完成
- `[x]` 已完成
- `[-]` 已取消

### 验证命令
每个任务完成后，运行以下命令验证：
```bash
gofmt -d .              # 检查格式
go vet ./...            # 静态分析
go test ./...           # 运行测试
go build ./cmd/...      # 构建验证
```

---

## 阶段一：测试基础设施 (P0)

> ✅ 已完成：共创建 17 个测试文件，覆盖核心模块

### 1.1 核心处理器测试

#### [x] Task 1.1.1: JSON 解析器单元测试
**文件**: `internal/core/prompt/json_parser_test.go`

创建 `json_parser_test.go`，测试 JSON 解析器：

```go
// 测试用例：
// 1. 正常的 JSON 代码块解析
// 2. 空 JSON 代码块
// 3. 无效 JSON 格式
// 4. 缺少 ```json``` 标记
// 5. 多个 JSON 代码块（取第一个）
// 6. 嵌套 JSON 结构
// 7. Unicode 字符处理
// 8. 特殊字符转义
```

验证：
- `extractJSONBlock()` 正确提取 JSON
- `parseJSONResponse()` 正确解析各种格式

#### [x] Task 1.1.2: 提示词引擎测试
**文件**: `internal/core/prompt/engine_test.go`

测试 `prompt.Engine` 的提示词生成：

```go
// 测试用例：
// 1. 音频转录任务提示词生成
// 2. 音频翻译任务提示词生成
// 3. 文本翻译任务提示词生成
// 4. 多目标语言提示词
// 5. 源语言指定场景
// 6. ParseResponse 成功解析
// 7. ParseResponse 解析失败处理
```

#### [x] Task 1.1.3: 语言管理器测试
**文件**: `internal/core/prompt/language_test.go`

测试语言配置和别名解析：

```go
// 测试用例：
// 1. 语言代码查找 (zh, en, ja)
// 2. 语言别名解析 (chinese -> zh)
// 3. 不存在的语言代码
// 4. 大小写不敏感匹配
// 5. 语言列表获取
// 6. 语言配置合并策略
// 7. 语言配置覆盖策略
```

---

### 1.2 LLM 管理器测试

#### [x] Task 1.2.1: LLM Manager 单元测试
**文件**: `internal/core/llm/manager_test.go`

使用 Mock 后端测试 LLM 管理器：

```go
// 测试用例：
// 1. 创建管理器成功
// 2. 创建管理器失败（无后端配置）
// 3. 创建管理器失败（未知后端类型）
// 4. Process 成功处理
// 5. Process 后端失败
// 6. GetBackend 获取存在的后端
// 7. GetBackend 获取不存在的后端
// 8. ListBackends 列出所有后端
// 9. HealthCheck 健康检查
```

创建 Mock 后端：
```go
type MockBackend struct {
    name        string
    shouldFail  bool
    response    *LLMResponse
}
```

#### [x] Task 1.2.2: 负载均衡器测试
**文件**: `internal/core/llm/loadbalancer_test.go`

测试轮询负载均衡器：

```go
// 测试用例：
// 1. 单后端选择
// 2. 多后端轮询顺序
// 3. 空后端列表错误处理
// 4. 并发选择安全性
// 5. AddBackend 添加后端
// 6. ReportSuccess/ReportError 记录
```

---

### 1.3 认证模块测试

#### [x] Task 1.3.1: API Key 认证测试
**文件**: `pkg/auth/auth_test.go`

测试 API Key 认证器：

```go
// 测试用例：
// 1. 有效 API Key 认证成功
// 2. 无效 API Key 认证失败
// 3. 空 API Key 认证失败
// 4. 限流配置正确设置
// 5. 无限制 Key (RequestsPerMinute = -1)
// 6. 服务类型身份识别
```

#### [x] Task 1.3.2: 多重认证器测试
**文件**: `pkg/auth/multi_auth_test.go`

测试 MultiAuthenticator：

```go
// 测试用例：
// 1. API Key 类型自动检测
// 2. JWT 类型自动检测
// 3. 匿名类型回退
// 4. 不支持的认证类型错误
// 5. 禁用的策略不加载
```

#### [x] Task 1.3.3: KeyStore 测试
**文件**: `pkg/auth/keystore_test.go`

测试密钥存储：

```go
// 测试用例：
// 1. 从文件加载密钥
// 2. 文件不存在处理
// 3. 无效 JSON 处理
// 4. GetKey 获取存在的密钥
// 5. GetKey 获取不存在的密钥
// 6. ListKeys 列出密钥（掩码）
```

---

### 1.4 处理器测试

#### [x] Task 1.4.1: 音频处理器测试
**文件**: `internal/core/audio/processor_test.go`

测试音频处理器：

```go
// 测试用例：
// 1. Validate 有效请求
// 2. Validate 空音频数据
// 3. Validate 超大音频文件
// 4. Validate 不支持的格式
// 5. Validate 无效任务类型
// 6. BuildLLMRequest 构建请求
// 7. BuildSuccessResponse 构建响应
// 8. GetCapabilities 返回能力
// 9. GetSupportedLanguages 返回语言
```

#### [x] Task 1.4.2: 文本处理器测试
**文件**: `internal/core/text/processor_test.go`

测试文本处理器：

```go
// 测试用例：
// 1. Validate 有效请求
// 2. Validate 空文本
// 3. Validate 超长文本
// 4. Validate 无目标语言
// 5. BuildLLMRequest 构建请求
// 6. BuildSuccessResponse 构建响应
```

#### [x] Task 1.4.3: 音频转换器测试
**文件**: `internal/core/audio/converter_test.go`

测试 FFmpeg 音频转换：

```go
// 测试用例：
// 1. WAV 格式不转换
// 2. OPUS 转 WAV
// 3. MP3 转 WAV
// 4. 无效音频数据处理
// 5. FFmpeg 不可用处理
```

---

### 1.5 处理服务测试

#### [x] Task 1.5.1: Processing Service 测试
**文件**: `internal/core/processing/service_test.go`

测试通用处理服务：

```go
// 测试用例：
// 1. 完整处理流程成功
// 2. Validate 失败
// 3. BuildLLMRequest 失败
// 4. LLM Process 失败
// 5. ParseResponse 失败
// 6. 处理时间记录
```

创建 Mock LogicHandler：
```go
type MockLogicHandler struct {
    validateErr    error
    buildErr       error
    response       *audio.ProcessResponse
}
```

---

### 1.6 API Handler 测试

#### [x] Task 1.6.1: Handler 单元测试
**文件**: `internal/api/handlers/handlers_test.go`

使用 httptest 测试 Handler：

```go
// 测试用例：
// 1. HealthCheck 基本响应
// 2. HealthCheck 详细模式
// 3. GetCapabilities 返回能力
// 4. ListSupportedLanguages 返回语言
// 5. ProcessAudioJSON 成功处理
// 6. ProcessAudioJSON 无认证
// 7. ProcessAudioJSON 无效 JSON
// 8. ProcessAudioJSON 无效 Base64
// 9. ProcessText 成功处理
// 10. ProcessText 无目标语言
// 11. GetMetrics 需要服务身份
```

#### [x] Task 1.6.2: Middleware 测试
**文件**: `internal/api/middleware/middleware_test.go`

测试中间件：

```go
// 测试用例：
// 1. Auth 中间件有效 Key
// 2. Auth 中间件无效 Key
// 3. Auth 中间件无 Key
// 4. RequestID 生成
// 5. RequestID 透传
// 6. Logging 记录请求
// 7. Recovery 捕获 panic
// 8. Metrics 记录指标
```

---

## 阶段二：代码质量改进 (P1)

### 2.1 错误处理标准化

#### [x] Task 2.1.1: 定义错误类型
**文件**: `internal/core/errors/errors.go`（新建）

创建统一的错误类型：

```go
package errors

type ErrorCode string

const (
    ErrCodeValidation   ErrorCode = "VALIDATION_ERROR"
    ErrCodeAuth         ErrorCode = "AUTH_ERROR"
    ErrCodeLLM          ErrorCode = "LLM_ERROR"
    ErrCodeParsing      ErrorCode = "PARSING_ERROR"
    ErrCodeInternal     ErrorCode = "INTERNAL_ERROR"
)

type AppError struct {
    Code    ErrorCode
    Message string
    Cause   error
    Details map[string]interface{}
}

func (e *AppError) Error() string
func (e *AppError) Unwrap() error
func NewValidationError(msg string, cause error) *AppError
func NewAuthError(msg string, cause error) *AppError
func NewLLMError(msg string, cause error) *AppError
func NewParsingError(msg string, cause error) *AppError
```

#### [x] Task 2.1.2: 重构 Handler 错误响应
**文件**: `internal/api/handlers/handlers.go`

统一错误响应格式：

```go
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code,omitempty"`
    Details any    `json:"details,omitempty"`
}

func respondError(c *gin.Context, status int, err error) {
    // 根据错误类型返回适当的响应
}
```

#### [x] Task 2.1.3: 重构各模块错误返回
**文件**: 多个文件

将各模块的 `fmt.Errorf` 替换为自定义错误类型：
- `internal/core/audio/processor.go`
- `internal/core/text/processor.go`
- `internal/core/llm/manager.go`
- `internal/core/prompt/engine.go`

---

### 2.2 代码问题修复

#### [x] Task 2.2.1: 修复硬编码时间戳
**文件**: `internal/api/handlers/handlers.go`

```go
// 修改前:
func getCurrentTimestamp() int64 {
    return 1704067200 // 示例时间戳
}

// 修改后:
func getCurrentTimestamp() int64 {
    return time.Now().Unix()
}
```

#### [x] Task 2.2.2: 实现 Webhook 认证
**文件**: `pkg/auth/auth.go`

实现 WebhookAuthenticator.Authenticate：

```go
func (auth *WebhookAuthenticator) Authenticate(ctx context.Context, credentials Credentials) (*Identity, error) {
    // 1. 构建 HTTP 请求到 webhook endpoint
    // 2. 发送凭据
    // 3. 解析响应获取身份信息
    // 4. 处理超时和错误
}
```

#### [x] Task 2.2.3: 实现异步处理状态查询
**文件**: `internal/api/handlers/handlers.go` 和新建 `internal/core/processing/status.go`

实现 GetProcessingStatus：

```go
// 1. 创建 StatusStore 接口
type StatusStore interface {
    Get(requestID string) (*ProcessingStatus, error)
    Set(requestID string, status *ProcessingStatus) error
}

// 2. 实现内存存储（可选 Redis）
type InMemoryStatusStore struct {
    store sync.Map
    ttl   time.Duration
}

// 3. 更新 Handler 使用 StatusStore
```

---

### 2.3 日志改进

#### [x] Task 2.3.1: 统一日志字段
**文件**: 多个文件

定义标准日志字段：

```go
// pkg/logging/fields.go (新建)
const (
    FieldRequestID   = "request_id"
    FieldUserID      = "user_id"
    FieldBackend     = "backend"
    FieldDuration    = "duration_ms"
    FieldAudioFormat = "audio_format"
    FieldTask        = "task"
)
```

更新所有日志调用使用标准字段。

#### [x] Task 2.3.2: 添加请求追踪
**文件**: `internal/api/middleware/middleware.go`

确保 request_id 传递到所有下游组件：

```go
// 1. 从 context 获取 request_id
// 2. 传递到 LLM Manager
// 3. 传递到 Processing Service
// 4. 在所有日志中包含
```

---

### 2.4 配置验证

#### [x] Task 2.4.1: 启动时配置验证
**文件**: `internal/config/config.go`

添加配置验证函数：

```go
func (c *Config) Validate() error {
    var errs []error
    
    // 验证服务器配置
    if c.Server.Port < 1 || c.Server.Port > 65535 {
        errs = append(errs, fmt.Errorf("invalid server port: %d", c.Server.Port))
    }
    
    // 验证后端配置
    if len(c.Backends.Providers) == 0 {
        errs = append(errs, fmt.Errorf("no backend providers configured"))
    }
    
    for _, provider := range c.Backends.Providers {
        if provider.URL == "" {
            errs = append(errs, fmt.Errorf("backend %s: missing URL", provider.Name))
        }
        // 验证 URL 格式
        if _, err := url.Parse(provider.URL); err != nil {
            errs = append(errs, fmt.Errorf("backend %s: invalid URL: %v", provider.Name, err))
        }
    }
    
    // 验证认证配置
    // ...
    
    return errors.Join(errs...)
}
```

#### [x] Task 2.4.2: 配置热重载支持
**文件**: `internal/config/config.go`

添加配置热重载：

```go
type ConfigWatcher struct {
    config   *Config
    onChange func(*Config)
    mu       sync.RWMutex
}

func (w *ConfigWatcher) Watch(path string) error {
    // 使用 fsnotify 监听配置文件变化
    // 重新加载并验证配置
    // 调用 onChange 回调
}
```

---

## 阶段三：性能优化 (P2)

### 3.1 内存优化

#### [x] Task 3.1.1: 音频数据池化
**文件**: `internal/core/audio/processor.go`

使用 sync.Pool 减少内存分配：

```go
var audioBufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 0, 1024*1024) // 1MB 初始容量
    },
}

func (p *Processor) processAudio(data []byte) ([]byte, error) {
    buf := audioBufferPool.Get().([]byte)
    defer audioBufferPool.Put(buf[:0])
    // 使用 buf 处理音频
}
```

#### [x] Task 3.1.2: 响应对象复用
**文件**: `internal/core/processing/service.go`

```go
var responsePool = sync.Pool{
    New: func() interface{} {
        return &ProcessResponse{
            Translations: make(map[string]string),
            Metadata:     make(map[string]interface{}),
        }
    },
}
```

---

### 3.2 并发优化

#### [x] Task 3.2.1: LLM 请求超时控制
**文件**: `internal/core/llm/manager.go`

添加可配置的超时：

```go
type ManagerConfig struct {
    RequestTimeout time.Duration
    RetryAttempts  int
    RetryDelay     time.Duration
}

func (m *Manager) ProcessWithTimeout(ctx context.Context, req *LLMRequest) (*LLMResponse, error) {
    ctx, cancel := context.WithTimeout(ctx, m.config.RequestTimeout)
    defer cancel()
    
    return m.Process(ctx, req)
}
```

#### [x] Task 3.2.2: 批量翻译优化
**文件**: `internal/core/text/processor.go`

支持批量文本翻译：

```go
type BatchProcessRequest struct {
    Texts           []string
    TargetLanguages []string
}

func (p *Processor) ProcessBatch(ctx context.Context, req BatchProcessRequest) ([]ProcessResponse, error) {
    // 使用 errgroup 并发处理
    // 或合并为单个 LLM 请求
}
```

---

### 3.3 缓存

#### [x] Task 3.3.1: 翻译结果缓存
**文件**: `internal/core/cache/cache.go`（新建）

```go
type TranslationCache interface {
    Get(key string) (*CachedTranslation, bool)
    Set(key string, value *CachedTranslation, ttl time.Duration)
}

type InMemoryCache struct {
    store   sync.Map
    maxSize int
}

// 缓存键生成
func GenerateCacheKey(text string, targetLangs []string) string {
    // 使用 hash 生成唯一键
}
```

#### [x] Task 3.3.2: 语言配置缓存
**文件**: `internal/core/prompt/language.go`

缓存语言查找结果：

```go
type LanguageManager struct {
    languages    []LanguageConfig
    codeIndex    map[string]*LanguageConfig  // 缓存 code -> config
    aliasIndex   map[string]*LanguageConfig  // 缓存 alias -> config
}
```

---

## 阶段四：可观测性 (P2)

### 4.1 指标增强

#### [x] Task 4.1.1: Prometheus 指标
**文件**: `pkg/metrics/prometheus.go`（新建）

```go
var (
    httpRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "lingualink_http_requests_total",
            Help: "Total HTTP requests",
        },
        []string{"method", "path", "status"},
    )
    
    llmRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "lingualink_llm_request_duration_seconds",
            Help:    "LLM request duration",
            Buckets: prometheus.DefBuckets,
        },
        []string{"backend", "model"},
    )
    
    audioProcessingDuration = prometheus.NewHistogram(
        prometheus.HistogramOpts{
            Name:    "lingualink_audio_processing_seconds",
            Help:    "Audio processing duration",
            Buckets: prometheus.DefBuckets,
        },
    )
)
```

#### [x] Task 4.1.2: 业务指标
**文件**: `pkg/metrics/business.go`（新建）

```go
// 业务相关指标
var (
    translationsTotal = prometheus.NewCounterVec(...)
    transcriptionsTotal = prometheus.NewCounterVec(...)
    languagePairUsage = prometheus.NewCounterVec(...)
    jsonParseSuccessRate = prometheus.NewGaugeVec(...)
)
```

---

### 4.2 健康检查增强

#### [x] Task 4.2.1: 深度健康检查
**文件**: `internal/api/handlers/handlers.go`

```go
type HealthStatus struct {
    Status      string                  `json:"status"`
    Timestamp   int64                   `json:"timestamp"`
    Version     string                  `json:"version"`
    Uptime      string                  `json:"uptime"`
    Components  map[string]ComponentHealth `json:"components,omitempty"`
}

type ComponentHealth struct {
    Status  string `json:"status"`
    Latency int64  `json:"latency_ms,omitempty"`
    Message string `json:"message,omitempty"`
}

func (h *Handler) DeepHealthCheck(c *gin.Context) {
    // 检查所有 LLM 后端
    // 检查 FFmpeg 可用性
    // 检查配置文件可读性
    // 返回详细状态
}
```

#### [x] Task 4.2.2: 就绪检查
**文件**: `internal/api/handlers/handlers.go`

```go
// GET /api/v1/ready
func (h *Handler) ReadinessCheck(c *gin.Context) {
    // 检查服务是否准备好接收请求
    // 至少一个 LLM 后端可用
    // 配置已加载
}

// GET /api/v1/live
func (h *Handler) LivenessCheck(c *gin.Context) {
    // 简单的存活检查
    // 服务进程正在运行
}
```

---

## 阶段五：代码组织优化 (P3)

### 5.1 接口抽象

#### [x] Task 5.1.1: 定义核心接口文件
**文件**: `internal/core/interfaces.go`（新建）

集中定义核心接口：

```go
package core

// Processor 通用处理器接口
type Processor interface {
    Process(ctx context.Context, req any) (any, error)
    Validate(req any) error
}

// Backend LLM 后端接口
type Backend interface {
    Process(ctx context.Context, req *LLMRequest) (*LLMResponse, error)
    HealthCheck(ctx context.Context) error
    GetName() string
}

// Cache 缓存接口
type Cache interface {
    Get(key string) (any, bool)
    Set(key string, value any, ttl time.Duration)
    Delete(key string)
}
```

### 5.2 测试工具

#### [x] Task 5.2.1: 测试助手函数
**文件**: `internal/testutil/helpers.go`（新建）

```go
package testutil

// NewTestLogger 创建测试用日志器
func NewTestLogger() *logrus.Logger

// NewMockLLMBackend 创建 Mock LLM 后端
func NewMockLLMBackend(name string, response *llm.LLMResponse) llm.LLMBackend

// LoadTestAudio 加载测试音频文件
func LoadTestAudio(t *testing.T, filename string) []byte

// AssertJSONEqual 比较 JSON 响应
func AssertJSONEqual(t *testing.T, expected, actual string)
```

#### [x] Task 5.2.2: 集成测试框架
**文件**: `internal/testutil/integration.go`（新建）

```go
package testutil

// TestServer 集成测试服务器
type TestServer struct {
    Server *httptest.Server
    Client *http.Client
    Config *config.Config
}

func NewTestServer(t *testing.T) *TestServer

func (ts *TestServer) DoRequest(method, path string, body any) (*http.Response, error)

func (ts *TestServer) Cleanup()
```

---

## 阶段六：文档和注释 (P3)

### 6.1 代码注释

#### [x] Task 6.1.1: 导出函数注释
所有导出的函数、类型、常量添加 godoc 风格注释：

```go
// Process 处理音频转录或翻译请求。
// 
// 参数:
//   - ctx: 请求上下文，用于超时和取消控制
//   - req: 音频处理请求，包含音频数据和任务类型
//
// 返回:
//   - ProcessResponse: 处理结果，包含转录文本和翻译
//   - error: 处理过程中的错误
//
// 示例:
//
//	resp, err := processor.Process(ctx, audio.ProcessRequest{
//	    Audio:       audioData,
//	    AudioFormat: "opus",
//	    Task:        "translate",
//	    TargetLanguages: []string{"en", "ja"},
//	})
func (p *Processor) Process(ctx context.Context, req ProcessRequest) (*ProcessResponse, error)
```

#### [x] Task 6.1.2: 包级文档
为每个包添加 `doc.go` 文件：

```go
// Package audio 提供音频处理功能，包括格式转换和 LLM 转录。
//
// 主要类型：
//   - Processor: 音频处理器，实现 LogicHandler 接口
//   - Converter: 音频格式转换器，使用 FFmpeg
//
// 使用示例：
//
//	processor := audio.NewProcessor(config, llmManager, promptEngine, logger)
//	response, err := processor.Process(ctx, request)
package audio
```

---

## 执行检查清单

完成每个任务后，确保：

- [ ] 代码通过 `gofmt` 格式化
- [ ] 代码通过 `go vet ./...` 检查
- [ ] 新代码有相应的测试
- [ ] 测试通过 `go test ./...`
- [ ] 构建成功 `go build ./cmd/...`
- [ ] 相关文档已更新

---

## 进度追踪

| 阶段 | 任务数 | 完成数 | 进度 |
|-----|-------|-------|------|
| 阶段一：测试基础设施 | 14 | 14 | 100% |
| 阶段二：代码质量改进 | 10 | 10 | 100% |
| 阶段三：性能优化 | 6 | 6 | 100% |
| 阶段四：可观测性 | 4 | 4 | 100% |
| 阶段五：代码组织优化 | 3 | 3 | 100% |
| 阶段六：文档和注释 | 2 | 2 | 100% |
| **总计** | **39** | **39** | **100%** |

---

*最后更新: 2026-01-23*
