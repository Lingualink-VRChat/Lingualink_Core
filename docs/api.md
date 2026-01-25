# Lingualink Core API 参考文档

## 概述

Lingualink Core API 提供高性能的多语言音频转录和翻译服务。API 遵循 RESTful 设计原则，所有请求和响应均为 `application/json` 格式。

**基础 URL**: `http://localhost:8080/api/v1`

---

## 认证

所有受保护的端点都需要通过 API 密钥进行认证。请将密钥包含在 HTTP 请求头中：

```http
X-API-Key: your-api-key-here
```

默认开发密钥: `lingualink-demo-key`

---

## 核心概念

### 任务类型 (Task)

`task` 参数决定系统对音频执行的具体操作：

| 任务 | 说明 | 响应行为 |
|-----|------|---------|
| `transcribe` | 仅转录 | `transcription` 有内容，`translations` 为空对象 |
| `translate` | 转录+翻译 | `transcription` 和 `translations` 均有内容 |

### 支持的音频格式

- WAV, MP3, M4A, OPUS, FLAC
- AAC, WMA, OGG, AMR, 3GP

音频数据需要 Base64 编码后传输。

---

## 端点参考

### `GET /health`

检查服务健康状态。

**认证**: 无需

**请求示例**:
```bash
curl -X GET "http://localhost:8080/api/v1/health"
```

**响应示例** (200 OK):
```json
{
    "status": "healthy",
    "timestamp": 1721835600,
    "version": "1.0.0"
}
```

---

### `GET /languages`

获取系统支持的语言列表。

**认证**: 无需

**请求示例**:
```bash
curl -X GET "http://localhost:8080/api/v1/languages"
```

**响应示例** (200 OK):
```json
{
    "languages": [
        {
            "code": "zh",
            "display": "中文",
            "english": "Chinese",
            "native": "中文",
            "aliases": ["chinese", "中文", "汉语", "zh-cn"]
        },
        {
            "code": "en",
            "display": "英文",
            "english": "English",
            "native": "English",
            "aliases": ["english", "英文", "英语"]
        }
    ],
    "count": 35
}
```

---

### `GET /capabilities`

获取系统能力、支持格式和限制。

**认证**: 无需

**请求示例**:
```bash
curl -X GET "http://localhost:8080/api/v1/capabilities"
```

**响应示例** (200 OK):
```json
{
    "audio_conversion": true,
    "max_audio_size": 33554432,
    "supported_formats": [
        "wav", "mp3", "m4a", "flac", "opus",
        "aac", "wma", "ogg", "amr", "3gp"
    ],
    "supported_languages": ["zh", "en", "ja", "ko", "es", "fr", "de"],
    "supported_tasks": ["translate", "transcribe"]
}
```

---

### `POST /process_audio`

对音频执行转录或翻译任务。

**认证**: 需要 (`X-API-Key`)

**请求体字段**:

| 字段 | 类型 | 必须 | 说明 |
|-----|------|-----|------|
| `audio` | string | **是** | Base64 编码的音频数据 |
| `audio_format` | string | **是** | 音频格式，如 "opus", "wav", "mp3" |
| `task` | string | **是** | `"translate"` 或 `"transcribe"` |
| `target_languages` | string[] | 翻译时必须 | 目标语言代码数组 |
| `source_language` | string | 否 | 源语言代码，可提高识别准确性 |

#### 示例 1: 翻译任务

**请求**:
```bash
curl -X POST "http://localhost:8080/api/v1/process_audio" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: lingualink-demo-key" \
  -d '{
    "audio": "T2dnUwACAAAAAAA...",
    "audio_format": "opus",
    "task": "translate",
    "target_languages": ["en", "ja"]
  }'
```

**响应** (200 OK):
```json
{
    "request_id": "req_1721836201123456789",
    "status": "success",
    "transcription": "你好",
    "translations": {
        "en": "Hello",
        "ja": "こんにちは"
    },
    "processing_time": 1.85,
    "metadata": {
        "backend": "default",
        "model": "qwenOmni7",
        "conversion_applied": true,
        "original_format": "opus",
        "processed_format": "wav",
        "parser": "json",
        "parse_success": true
    }
}
```

#### 示例 2: 转录任务

**请求**:
```bash
curl -X POST "http://localhost:8080/api/v1/process_audio" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: lingualink-demo-key" \
  -d '{
    "audio": "UklGRiYAAABXQVZFZm10...",
    "audio_format": "wav",
    "task": "transcribe"
  }'
```

**响应** (200 OK):
```json
{
    "request_id": "req_1721836202234567890",
    "status": "success",
    "transcription": "这是一个纯转录测试。",
    "translations": {},
    "processing_time": 1.23,
    "metadata": {
        "backend": "default",
        "model": "qwenOmni7",
        "conversion_applied": false,
        "original_format": "wav",
        "processed_format": "wav",
        "parser": "json",
        "parse_success": true
    }
}
```

---

### `POST /process_text`

对文本执行翻译。

**认证**: 需要 (`X-API-Key`)

**请求体字段**:

| 字段 | 类型 | 必须 | 说明 |
|-----|------|-----|------|
| `text` | string | **是** | 要翻译的文本（最大 3000 字符）|
| `target_languages` | string[] | **是** | 目标语言代码数组 |
| `source_language` | string | 否 | 源文本语言代码 |

**请求示例**:
```bash
curl -X POST "http://localhost:8080/api/v1/process_text" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: lingualink-demo-key" \
  -d '{
    "text": "Lingualink is a powerful translation core.",
    "target_languages": ["zh", "ja"]
  }'
```

**响应示例** (200 OK):
```json
{
    "request_id": "txt_1721836501123456789",
    "status": "success",
    "source_text": "Lingualink is a powerful translation core.",
    "translations": {
        "zh": "Lingualink 是一个强大的翻译核心。",
        "ja": "Lingualinkは強力な翻訳コアです。"
    },
    "processing_time": 0.95,
    "metadata": {
        "backend": "default",
        "model": "qwenOmni7",
        "parser": "json",
        "parse_success": true
    }
}
```

---

### `GET /admin/metrics`

获取系统监控指标。

**认证**: 需要 (`X-API-Key`)

**响应**包含:
- HTTP 请求延迟
- 处理成功/失败计数
- 后端健康状态

---

## 错误处理

当请求无法处理时，API 返回非 200 状态码和错误信息：

| 状态码 | 含义 | 示例 |
|-------|------|------|
| `400 Bad Request` | 请求无效 | `{"error": "target languages are required"}` |
| `401 Unauthorized` | 认证失败 | `{"error": "authentication failed"}` |
| `413 Payload Too Large` | 请求体过大 | `{"error": "audio size exceeds maximum"}` |
| `500 Internal Server Error` | 服务器错误 | `{"error": "llm process failed"}` |

**错误响应格式**:
```json
{
    "error": "error message description"
}
```

---

## 响应字段说明

### 通用字段

| 字段 | 类型 | 说明 |
|-----|------|------|
| `request_id` | string | 请求唯一标识符 |
| `status` | string | `"success"` 或 `"error"` |
| `processing_time` | float | 处理耗时（秒）|
| `metadata` | object | 处理元数据 |

### metadata 字段

| 字段 | 类型 | 说明 |
|-----|------|------|
| `backend` | string | 使用的 LLM 后端名称 |
| `model` | string | 使用的模型名称 |
| `parser` | string | 使用的解析器 (`"json"`) |
| `parse_success` | boolean | 解析是否成功 |
| `conversion_applied` | boolean | 是否应用了音频格式转换 |
| `original_format` | string | 原始音频格式 |
| `processed_format` | string | 处理后的音频格式 |

---

## SDK 与客户端库

### cURL 测试

```bash
# 健康检查
curl http://localhost:8080/api/v1/health

# 音频处理（从文件读取）
AUDIO_BASE64=$(base64 -w 0 test/test.opus)
curl -X POST "http://localhost:8080/api/v1/process_audio" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: lingualink-demo-key" \
  -d "{\"audio\": \"$AUDIO_BASE64\", \"audio_format\": \"opus\", \"task\": \"translate\", \"target_languages\": [\"en\", \"ja\"]}"
```

### 测试脚本

使用项目提供的测试脚本：

```bash
./test_api.sh
```
