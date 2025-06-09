
好的，我已经仔细分析了您提供的所有项目文件，包括 Go 源代码、配置文件、API 文档和各种脚本。现有的文档，特别是 `README.md`、`功能实现.md` 和 `API_Documentation.md`，已经部分过时，未能完全体现当前代码库的先进架构和功能。

我将为您生成一份全面、准确且现代化的项目总结文档，以替换旧的文档。这份新文档将作为项目的核心 `README.md`，准确反映其当前的设计和功能。

---

# Lingualink Core - 综合系统文档

## 1. 项目概述

Lingualink Core 是一款专为多语言、多模态处理设计的高性能后端服务。项目采用 Go 语言构建，核心功能是提供实时的音频转录、翻译以及纯文本翻译服务。其架构设计精良，具备高度的可扩展性、可配置性和生产环境部署能力。

该系统不仅仅是一个简单的 API 服务，而是一个完整、强大的处理引擎，集成了动态提示词工程、可插拔的 LLM 后端管理、多策略认证和强大的音频处理能力。

## 2. 核心特性

- **多模态处理**:
  - **音频处理**: 支持多种格式 (`wav`, `mp3`, `opus`, `m4a`, `flac` 等) 的音频转录与翻译。
  - **文本处理**: 支持纯文本的多语言翻译。

- **先进的 LLM 集成**:
  - **多后端支持**: 可同时接入多个 OpenAI 兼容的后端（如 VLLM, Groq, Together AI 等）。
  - **负载均衡**: 内置轮询（Round-Robin）策略，自动在多个健康后端之间分发请求。
  - **健康检查**: 自动监控后端状态，确保服务高可用。

- **强大的提示词引擎**:
  - **动态模板**: 服务端根据请求任务动态生成最优化的提示词。
  - **语言管理**: 通过配置文件管理语言（支持别名和多语言名称），而非硬编码。
  - **智能响应解析**: 能够将 LLM 返回的半结构化文本智能解析为标准化的 JSON 格式，并具备回退机制。

- **企业级架构**:
  - **模块化设计**: 清晰的关注点分离，易于维护和扩展。
  - **通用处理服务**: 抽象出统一的处理流程，可快速扩展支持新的处理类型（如图像、视频）。
  - **多策略认证**: 支持 API Key、JWT 等多种认证方式，并可通过配置启用或禁用。
  - **全面的配置系统**: 使用 Viper 进行配置管理，支持 YAML 文件和环境变量，灵活性极高。

- **生产级运维能力**:
  - **容器化部署**: 提供 `Dockerfile` 和 `docker-compose.yml`，实现一键部署。
  - **结构化日志**: 使用 Logrus 输出 JSON 格式日志，便于采集和分析。
  - **内置监控**: 提供基础的性能指标（延迟、QPS）收集，并提供 API 端点查询。
  - **丰富的开发工具**: 包含完整的启动、停止、测试和开发辅助脚本。

## 3. 系统架构

### 3.1 总体架构图

```mermaid
graph TD
    subgraph "客户端"
        A[用户应用 / SDK]
    end

    subgraph "Lingualink Core"
        B[API层 (Gin)] -- JSON/HTTP --> C{通用处理服务}
        C -- 分发任务 --> D{音频/文本处理器}
        D -- 构建请求 --> E[提示词引擎]
        E -- 生成Prompt --> F[LLM管理器]
        F -- 选择后端 --> G[LLM后端池]
        G -- 请求 --> H[LLM服务 (VLLM/OpenAI)]
        H -- LLM响应 --> F
        F -- 返回响应 --> E
        E -- 解析响应 --> D
        D -- 构建最终结果 --> C
        C -- 返回JSON --> B
    end
    
    subgraph "外部依赖"
        H
        I[FFmpeg]
    end

    subgraph "可插拔模块"
        J[认证模块]
        K[配置模块]
        L[监控模块]
    end

    A --> B
    D -.-> I
    B <--> J
    style C fill:#bbf,stroke:#333,stroke-width:2px
    style E fill:#bbf,stroke:#333,stroke-width:2px
    style F fill:#bbf,stroke:#333,stroke-width:2px
```

### 3.2 请求处理生命周期

1.  **API 接收**: Gin 框架接收到 HTTP 请求（例如 `POST /api/v1/process_audio`）。
2.  **中间件处理**: 请求依次通过 `CORS`, `RequestID`, `Logging`, `Metrics`, `Recovery` 和 `Auth` 中间件。认证中间件验证凭据，并将身份信息 `identity` 存入请求上下文。
3.  **路由到处理器**: 请求被路由到 `handlers.go` 中的具体处理函数（如 `ProcessAudioJSON`）。
4.  **调用通用服务**: 处理器调用 `processing.Service` 的 `Process` 方法，这是一个泛型方法，统一了处理流程。
5.  **逻辑处理**: `processing.Service` 依次调用特定逻辑处理器（`audio.Processor` 或 `text.Processor`）的接口方法：
    a.  `Validate()`: 验证输入参数（如文件大小、格式、文本长度）。
    b.  `BuildLLMRequest()`:
        -   **音频**: 如果需要，调用 `audio.Converter` 使用 FFmpeg 将音频转换为 WAV 格式。
        -   调用 `prompt.Engine` 构建动态的、针对特定任务的系统和用户提示词，并生成输出规则。
        -   组装成 `llm.LLMRequest`。
    c.  `llm.Manager` 的 `Process` 方法被调用，它通过负载均衡器选择一个健康的 LLM 后端发送请求。
6.  **响应解析**:
    a.  `prompt.Engine` 的 `ParseResponse` 方法被调用，使用 `StructuredParser` 将 LLM 的文本响应解析为键值对。
    b.  解析器将 LLM 输出的语言名称（如“英文”）映射回标准语言代码（如“en”）。
7.  **构建最终响应**:
    a.  `BuildSuccessResponse()`: 将解析后的内容组装成最终的 API 响应结构体。
    b.  `ApplyFallback()`: 如果解析失败或结果不完整，执行回退逻辑（例如，将整个原始响应作为翻译结果）。
8.  **返回结果**: 最终的 JSON 响应通过 API 层返回给客户端。

## 4. API 参考 (v2.0)

**基础 URL**: `http://localhost:8080/api/v1`  
**认证**: 所有受保护的端点都需要在请求头中提供 `X-API-Key: your-api-key`。

---

### **GET /health**
检查服务健康状态。
- **认证**: 无需
- **成功响应 (200 OK)**:
  ```json
  { "status": "healthy", "timestamp": 1704067200, "version": "1.0.0" }
  ```

---

### **GET /capabilities**
获取系统支持的功能、格式和限制。
- **认证**: 需要
- **成功响应 (200 OK)**:
  ```json
  {
    "supported_formats": ["wav", "mp3", "m4a", "flac", "opus", ...],
    "max_audio_size": 33554432,
    "supported_tasks": ["translate", "transcribe"],
    "supported_languages": ["zh", "en", "ja", ...],
    "audio_conversion": true
  }
  ```

---

### **GET /languages**
获取所有支持的语言及其别名。
- **认证**: 需要
- **成功响应 (200 OK)**:
  ```json
  {
    "languages": [
      { "code": "en", "names": { "display": "英文" }, "aliases": ["english", "英语"] },
      { "code": "ja", "names": { "display": "日文" }, "aliases": ["japanese", "日语"] }
    ],
    "count": 10
  }
  ```

---

### **POST /process_audio**
处理音频转录和翻译。
- **认证**: 需要
- **请求体 (application/json)**:
  ```json
  {
    "audio": "<base64_encoded_string>",
    "audio_format": "opus",
    "task": "translate",
    "target_languages": ["en", "ja"]
  }
  ```
- **字段说明**:
  - `audio` (string, required): Base64 编码的音频数据。
  - `audio_format` (string, required): 音频原始格式 (e.g., "wav", "opus")。
  - `task` (string, required): 任务类型。`"transcribe"` (仅转录) 或 `"translate"` (转录并翻译)。
  - `target_languages` (array of strings, required for `translate` task): 目标语言代码数组。
- **成功响应 (200 OK)**:
  ```json
  {
    "request_id": "req_1720275899818816000",
    "status": "success",
    "transcription": "你好，这是一个测试。",
    "translations": {
      "en": "Hello, this is a test.",
      "ja": "こんにちは、これはテストです。"
    },
    "raw_response": "原文: 你好，这是一个测试。\n英文: Hello, this is a test.\n日文: こんにちは、これはテストです。",
    "processing_time": 2.15,
    "metadata": { ... }
  }
  ```

---

### **POST /process_text**
处理纯文本翻译。
- **认证**: 需要
- **请求体 (application/json)**:
  ```json
  {
    "text": "Hello, world!",
    "target_languages": ["zh", "ja"]
  }
  ```
- **字段说明**:
  - `text` (string, required): 需要翻译的文本。
  - `target_languages` (array of strings, required): 目标语言代码数组。
- **成功响应 (200 OK)**:
  ```json
  {
    "request_id": "txt_1720276135017081000",
    "status": "success",
    "source_text": "Hello, world!",
    "translations": {
      "zh": "你好，世界！",
      "ja": "こんにちは、世界！"
    },
    ...
  }
  ```

---

### **GET /admin/metrics**
获取内部监控指标。
- **认证**: 需要 (要求服务级别的 `identity.Type`)
- **成功响应 (200 OK)**:
  ```json
  {
    "counters": { "http_requests_total:method=POST...": 5 },
    "gauges": { ... },
    "latency": { "http_request_duration...": { "avg": 150.5, "count": 5 } }
  }
  ```

## 5. 快速开始与开发

### 5.1 环境要求
- Go 1.21+
- Docker & Docker Compose
- FFmpeg (用于音频格式转换)

### 5.2 配置文件
1.  **复制模板**:
    ```bash
    cp config/config.template.yaml config/config.yaml
    cp config/api_keys.template.json config/api_keys.json
    ```
2.  **编辑 `config/config.yaml`**:
    - 设置 `server.port`。
    - 配置 `backends.providers`，至少提供一个 LLM 后端的 `url` 和 `model`。
    - (可选) 调整 `prompt` 和 `logging` 配置。
3.  **编辑 `config/api_keys.json`**:
    - 添加或修改 API 密钥。密钥是 JSON 对象中的 key，值是该 key 的配置。
    - `requests_per_minute: -1` 表示无限制。

### 5.3 运行服务

**使用开发脚本 (推荐)**
```bash
# 检查环境并启动开发服务器 (自动处理端口冲突)
./start.sh --dev
```

**使用 Docker Compose**
```bash
# 构建并以守护进程模式启动
docker-compose up --build -d
```

### 5.4 测试
项目提供了丰富的测试脚本，用于快速验证各项功能。

```bash
# 快速健康检查
./quick_test.sh

# 完整的 API 功能测试 (包括错误处理)
./test_api.sh

# 音频处理专项测试 (多格式、多任务)
./test_audio.sh

# 文本翻译专项测试
./test_text_simple.sh
```

## 6. 如何扩展

### 6.1 添加新的 LLM 后端
1.  在 `internal/core/llm/backends.go` 中，创建一个新的 `struct` 并实现 `LLMBackend` 接口。
2.  在 `internal/core/llm/manager.go` 的 `NewManager` 函数中，添加一个 `case` 来实例化你的新后端。
3.  在 `config/config.yaml` 中添加新的 `provider` 配置，并指定新的 `type`。

### 6.2 添加新的认证方式
1.  在 `pkg/auth/auth.go` 中，创建一个新的 `struct` 并实现 `Authenticator` 接口。
2.  在 `NewMultiAuthenticator` 函数中，添加一个 `case` 来注册你的新认证器。
3.  在 `config/config.yaml` 的 `auth.strategies` 中添加新的策略配置。

---