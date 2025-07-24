
# Lingualink Core API 接口文档

## 1\. 概述

欢迎使用 Lingualink Core API 。本API旨在提供一个高性能、稳定且易于集成的多语言处理服务。API 遵循 RESTful 设计原则，所有请求和响应主体均为 `application/json` 格式。

  - **基础 URL**: `http://localhost:8080/api/v1`

### 1.1 认证

所有受保护的端点都需要通过 **API 密钥**进行认证。请将您的密钥包含在 HTTP 请求头中：

```http
X-API-Key: your-api-key-here
```

### 1.2 核心概念：任务类型 (Task)

`task` 参数是音频处理的核心，它决定了系统对音频执行的具体操作。

  - **`transcribe` (转录任务)**

      - **作用**: 仅将音频内容识别并转换成其原始语言的文本。
      - **行为**: 此任务**不会**进行翻译。在响应中，`transcription` 字段会包含转录结果，而 `translations` 字段将是一个**空对象** `{}`。
      - **适用场景**: 当您只需要语音转文字功能时使用。

  - **`translate` (翻译任务)**

      - **作用**: 对音频执行“先转录，后翻译”的流水线操作。
      - **行为**: 系统首先将音频转录为原文，然后将该原文翻译成您在 `target_languages` 中指定的所有目标语言。在响应中，`transcription` 和 `translations` 字段都会被填充。
      - **适用场景**: 当您需要完整的语音翻译功能时使用。

### 1.3 核心概念：优雅降级与部分成功

大语言模型（LLM）可能不会总是严格遵循格式化指令。为了避免因此导致请求完全失败，Lingualink Core 设计了\*\*优雅降级（Graceful Fallback）\*\*机制。

当系统无法从 LLM 的响应中解析出标准结构时，它会：

1.  返回 `200 OK` 状态码，确保连接成功。
2.  在响应体中将 `status` 字段标记为 **`partial_success`**。
3.  在 `metadata` 中添加 `fallback_mode: true` 和 `fallback_reason` 以说明原因。
4.  尽最大努力从 LLM 的原始响应中提取有效信息，并填充到 `transcription` 或 `translations` 字段中。

作为开发者，您应该检查 `status` 字段，以便在客户端对 `partial_success` 状态进行特殊处理（如UI提示）。

## 2\. 端点参考

-----

### **`GET /health`**

检查服务的健康状态。

  - **认证**: 无需
  - **示例请求**:
    ```bash
    curl -X GET "http://localhost:8080/api/v1/health"
    ```
  - **成功响应 (200 OK)**:
    ```json
    {
        "status": "healthy",
        "timestamp": 1721835600,
        "version": "1.0.0"
    }
    ```

-----

### **`GET /languages`**

获取系统当前支持的所有语言的详细列表。

  - **认证**: 无需
  - **示例请求**:
    ```bash
    curl -X GET "http://localhost:8080/api/v1/languages"
    ```
  - **成功响应 (200 OK)**:
    ```json
    {
        "languages": [
            {
                "aliases": ["chinese", "中文", "汉语", "zh-cn"],
                "code": "zh",
                "display": "中文",
                "english": "Chinese",
                "native": "中文"
            },
            {
                "aliases": ["english", "英文", "英语"],
                "code": "en",
                "display": "英文",
                "english": "English",
                "native": "English"
            }
            // ... more languages
        ],
        "count": 35
    }
    ```

-----

### **`GET /capabilities`**

获取系统的能力、支持格式和限制。

  - **认证**: 无需
  - **示例请求**:
    ```bash
    curl -X GET "http://localhost:8080/api/v1/capabilities"
    ```
  - **成功响应 (200 OK)**:
    ```json
    {
        "audio_conversion": true,
        "conversion_metrics": {
            // ... 音频转换器相关的详细指标
        },
        "max_audio_size": 33554432,
        "supported_formats": [
            "wav", "mp3", "m4a", "flac", "opus", 
            "aac", "wma", "ogg", "amr", "3gp"
        ],
        "supported_languages": ["zh", "en", "ja", /* ... */],
        "supported_tasks": ["translate", "transcribe"]
    }
    ```

-----

### **`POST /process_audio`**

对提供的音频数据执行转录或翻译任务。

  - **认证**: **需要** (`X-API-Key`)
  - **请求体字段**:
    | 字段名 | 类型 | 是否必须 | 描述 |
    | :--- | :--- | :--- | :--- |
    | `audio` | `string` | **是** | Base64 编码的音频数据。 |
    | `audio_format` | `string` | **是** | 音频的原始格式，例如 "opus", "wav", "mp3"。 |
    | `task` | `string` | **是** | 要执行的任务。**`"translate"`** 或 **`"transcribe"`**。 |
    | `target_languages`|`array of strings`| 任务为 `translate` 时**是** | 目标语言的代码数组，例如 `["en", "ja"]`。 |
    | `source_language` | `string` | 否 | 音频的源语言代码。如果提供，可以提高识别准确性。 |

#### 示例 1: 翻译任务 (`task: "translate"`)

此示例演示了如何将一段`opus`格式的音频（内容为"你好"）转录并翻译成英文和日文。

  - **请求**:

    ```bash
    curl -X POST "http://localhost:8080/api/v1/process_audio" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: dev-key-123" \
    -d '{
        "audio": "CwEADgA...AEU=", # 这是一个简短的 Base64 音频数据示例
        "audio_format": "opus",
        "task": "translate",
        "target_languages": ["en", "ja"]
    }'
    ```

  - **成功响应 (200 OK, `status: "success"`)**:

    ```json
    {
        "request_id": "req_1721836201123456789",
        "status": "success",
        "transcription": "你好",
        "translations": {
            "en": "Hello",
            "ja": "こんにちは"
        },
        "raw_response": "原文: 你好\n英文: Hello\n日文: こんにちは",
        "processing_time": 1.85,
        "metadata": {
            "backend": "default",
            "conversion_applied": true,
            "fallback_mode": false,
            "model": "qwenOmni7",
            "original_format": "opus",
            "processed_format": "wav"
        }
    }
    ```

#### 示例 2: 转录任务 (`task: "transcribe"`)

此示例演示了如何仅转录一段`wav`格式的音频，而不进行翻译。

  - **请求**:

    ```bash
    curl -X POST "http://localhost:8080/api/v1/process_audio" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: dev-key-123" \
    -d '{
        "audio": "UklGRiYAAABXQVZFZm10IBAAAAABAAEARKwAAIhYAQACABAAZGF0YQ...",
        "audio_format": "wav",
        "task": "transcribe"
    }'
    ```

  - **成功响应 (200 OK, `status: "success"`)**:
    注意 `translations` 字段是一个空对象。

    ```json
    {
        "request_id": "req_1721836202234567890",
        "status": "success",
        "transcription": "这是一个纯转录测试。",
        "translations": {},
        "raw_response": "原文: 这是一个纯转录测试。",
        "processing_time": 1.23,
        "metadata": {
            "backend": "default",
            "conversion_applied": false,
            "fallback_mode": false,
            "model": "qwenOmni7",
            "original_format": "wav",
            "processed_format": "wav"
        }
    }
    ```

#### 示例 3: 部分成功 (Fallback)

此场景模拟 LLM 未按规定格式返回数据，只返回了一句纯文本。

  - **假设 LLM 返回的 `raw_response` 是**: `"Hello this is a test from the system."`
  - **API 响应 (200 OK, `status: "partial_success"`)**:
    系统会触发回退逻辑，将原始响应同时作为转录和第一个目标语言的翻译。
    ```json
    {
        "request_id": "req_1721836203345678901",
        "status": "partial_success",
        "transcription": "Hello this is a test from the system.",
        "translations": {
            "en": "Hello this is a test from the system."
        },
        "raw_response": "Hello this is a test from the system.",
        "processing_time": 2.05,
        "metadata": {
            "backend": "default",
            "conversion_applied": true,
            "fallback_mode": true,
            "fallback_reason": "using sanitized raw content as transcription; using sanitized raw content as translation for en",
            "model": "qwenOmni7",
            "original_format": "opus",
            "processed_format": "wav"
        }
    }
    ```

-----

### **`POST /process_text`**

对提供的文本字符串执行翻译。

  - **认证**: **需要** (`X-API-Key`)
  - **请求体字段**:
    | 字段名 | 类型 | 是否必须 | 描述 |
    | :--- | :--- | :--- | :--- |
    | `text` | `string` | **是** | 要翻译的文本字符串。最大长度 3000 字符。 |
    | `target_languages`|`array of strings`| **是** | 目标语言的代码数组，例如 `["ja", "ko"]`。 |
    | `source_language` | `string` | 否 | 源文本的语言代码。 |

#### 示例 1: 文本翻译

  - **请求**:
    ```bash
    curl -X POST "http://localhost:8080/api/v1/process_text" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: dev-key-123" \
    -d '{
        "text": "Lingualink is a powerful translation core.",
        "target_languages": ["zh", "ja"]
    }'
    ```
  - **成功响应 (200 OK)**:
    ```json
    {
        "request_id": "txt_1721836501123456789",
        "status": "success",
        "source_text": "Lingualink is a powerful translation core.",
        "translations": {
            "zh": "Lingualink 是一个强大的翻译核心。",
            "ja": "Lingualinkは強力な翻訳コアです。"
        },
        "raw_response": "中文: Lingualink 是一个强大的翻译核心。\n日文: Lingualinkは強力な翻訳コアです。",
        "processing_time": 0.95,
        "metadata": {
            "backend": "default",
            "fallback_mode": false,
            "model": "qwenOmni7"
        }
    }
    ```

## 3\. 错误处理

当请求无法被处理时（例如，认证失败、输入无效），API会返回非 `200` 的 HTTP 状态码和包含错误信息的 JSON 对象。

| 状态码 | 含义 | 示例错误信息 |
| :--- | :--- | :--- |
| `400 Bad Request` | 请求无效 | `{"error": "target languages are required"}` |
| `401 Unauthorized` | 认证失败 | `{"error": "authentication failed"}` |
| `413 Payload Too Large`| 请求体过大 |`{"error": "audio size (35MB) exceeds maximum allowed size (32MB)"}`|
| `500 Internal Server Error`| 服务器内部错误 | `{"error": "llm process failed: backend process failed..."}` |

  - **错误响应示例**:
    ```bash
    # 请求缺少 target_languages
    curl -X POST "http://localhost:8080/api/v1/process_text" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: dev-key-123" \
    -d '{
        "text": "This will fail."
    }'
    ```
    ```json
    // HTTP/1.1 400 Bad Request
    {
        "error": "target languages are required"
    }
    ```